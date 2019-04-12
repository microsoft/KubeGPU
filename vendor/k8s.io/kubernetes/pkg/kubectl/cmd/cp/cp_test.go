/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cp

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest/fake"
	kexec "k8s.io/kubernetes/pkg/kubectl/cmd/exec"
	cmdtesting "k8s.io/kubernetes/pkg/kubectl/cmd/testing"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
)

type FileType int

const (
	RegularFile FileType = 0
	SymLink     FileType = 1
	RegexFile   FileType = 2
)

func TestExtractFileSpec(t *testing.T) {
	tests := []struct {
		spec              string
		expectedPod       string
		expectedNamespace string
		expectedFile      string
		expectErr         bool
	}{
		{
			spec:              "namespace/pod:/some/file",
			expectedPod:       "pod",
			expectedNamespace: "namespace",
			expectedFile:      "/some/file",
		},
		{
			spec:         "pod:/some/file",
			expectedPod:  "pod",
			expectedFile: "/some/file",
		},
		{
			spec:         "/some/file",
			expectedFile: "/some/file",
		},
		{
			spec:      ":file:not:exist:in:local:filesystem",
			expectErr: true,
		},
		{
			spec:      "namespace/pod/invalid:/some/file",
			expectErr: true,
		},
		{
			spec:         "pod:/some/filenamewith:in",
			expectedPod:  "pod",
			expectedFile: "/some/filenamewith:in",
		},
	}
	for _, test := range tests {
		spec, err := extractFileSpec(test.spec)
		if test.expectErr && err == nil {
			t.Errorf("unexpected non-error")
			continue
		}
		if err != nil && !test.expectErr {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if spec.PodName != test.expectedPod {
			t.Errorf("expected: %s, saw: %s", test.expectedPod, spec.PodName)
		}
		if spec.PodNamespace != test.expectedNamespace {
			t.Errorf("expected: %s, saw: %s", test.expectedNamespace, spec.PodNamespace)
		}
		if spec.File != test.expectedFile {
			t.Errorf("expected: %s, saw: %s", test.expectedFile, spec.File)
		}
	}
}

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/foo/bar",
			expected: "foo/bar",
		},
		{
			input:    "foo/bar",
			expected: "foo/bar",
		},
	}
	for _, test := range tests {
		out := getPrefix(test.input)
		if out != test.expected {
			t.Errorf("expected: %s, saw: %s", test.expected, out)
		}
	}
}

func TestStripPathShortcuts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "test single path shortcut prefix",
			input:    "../foo/bar",
			expected: "foo/bar",
		},
		{
			name:     "test multiple path shortcuts",
			input:    "../../foo/bar",
			expected: "foo/bar",
		},
		{
			name:     "test multiple path shortcuts with absolute path",
			input:    "/tmp/one/two/../../foo/bar",
			expected: "tmp/foo/bar",
		},
		{
			name:     "test multiple path shortcuts with no named directory",
			input:    "../../",
			expected: "",
		},
		{
			name:     "test multiple path shortcuts with no named directory and no trailing slash",
			input:    "../..",
			expected: "",
		},
		{
			name:     "test multiple path shortcuts with absolute path and filename containing leading dots",
			input:    "/tmp/one/two/../../foo/..bar",
			expected: "tmp/foo/..bar",
		},
		{
			name:     "test multiple path shortcuts with no named directory and filename containing leading dots",
			input:    "../...foo",
			expected: "...foo",
		},
		{
			name:     "test filename containing leading dots",
			input:    "...foo",
			expected: "...foo",
		},
		{
			name:     "test root directory",
			input:    "/",
			expected: "",
		},
	}

	for _, test := range tests {
		out := stripPathShortcuts(test.input)
		if out != test.expected {
			t.Errorf("expected: %s, saw: %s", test.expected, out)
		}
	}
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
}

func TestTarUntar(t *testing.T) {
	dir, err := ioutil.TempDir("", "input")
	checkErr(t, err)
	dir2, err := ioutil.TempDir("", "output")
	checkErr(t, err)
	dir3, err := ioutil.TempDir("", "dir")
	checkErr(t, err)

	dir = dir + "/"
	defer func() {
		os.RemoveAll(dir)
		os.RemoveAll(dir2)
		os.RemoveAll(dir3)
	}()

	files := []struct {
		name     string
		nameList []string
		data     string
		omitted  bool
		fileType FileType
	}{
		{
			name:     "foo",
			data:     "foobarbaz",
			fileType: RegularFile,
		},
		{
			name:     "dir/blah",
			data:     "bazblahfoo",
			fileType: RegularFile,
		},
		{
			name:     "some/other/directory/",
			data:     "with more data here",
			fileType: RegularFile,
		},
		{
			name:     "blah",
			data:     "same file name different data",
			fileType: RegularFile,
		},
		{
			name:     "gakki",
			data:     "tmp/gakki",
			fileType: SymLink,
		},
		{
			name:     "relative_to_dest",
			data:     path.Join(dir2, "foo"),
			fileType: SymLink,
		},
		{
			name:     "tricky_relative",
			data:     path.Join(dir3, "xyz"),
			omitted:  true,
			fileType: SymLink,
		},
		{
			name:     "absolute_path",
			data:     "/tmp/gakki",
			omitted:  true,
			fileType: SymLink,
		},
		{
			name:     "blah*",
			nameList: []string{"blah1", "blah2"},
			data:     "regexp file name",
			fileType: RegexFile,
		},
	}

	for _, file := range files {
		filepath := path.Join(dir, file.name)
		if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if file.fileType == RegularFile {
			createTmpFile(t, filepath, file.data)
		} else if file.fileType == SymLink {
			err := os.Symlink(file.data, filepath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		} else if file.fileType == RegexFile {
			for _, fileName := range file.nameList {
				createTmpFile(t, path.Join(dir, fileName), file.data)
			}
		} else {
			t.Fatalf("unexpected file type: %v", file)
		}
	}

	opts := NewCopyOptions(genericclioptions.NewTestIOStreamsDiscard())

	writer := &bytes.Buffer{}
	if err := makeTar(dir, dir, writer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader := bytes.NewBuffer(writer.Bytes())
	if err := opts.untarAll(reader, dir2, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, file := range files {
		absPath := filepath.Join(dir2, strings.TrimPrefix(dir, os.TempDir()))
		filePath := filepath.Join(absPath, file.name)

		if file.fileType == RegularFile {
			cmpFileData(t, filePath, file.data)
		} else if file.fileType == SymLink {
			dest, err := os.Readlink(filePath)
			if file.omitted {
				if err != nil && strings.Contains(err.Error(), "no such file or directory") {
					continue
				}
				t.Fatalf("expected to omit symlink for %s", filePath)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if file.data != dest {
				t.Fatalf("expected: %s, saw: %s", file.data, dest)
			}
		} else if file.fileType == RegexFile {
			for _, fileName := range file.nameList {
				cmpFileData(t, path.Join(dir, fileName), file.data)
			}
		} else {
			t.Fatalf("unexpected file type: %v", file)
		}
	}
}

func TestTarUntarWrongPrefix(t *testing.T) {
	dir, err := ioutil.TempDir("", "input")
	checkErr(t, err)
	dir2, err := ioutil.TempDir("", "output")
	checkErr(t, err)

	dir = dir + "/"
	defer func() {
		os.RemoveAll(dir)
		os.RemoveAll(dir2)
	}()

	filepath := path.Join(dir, "foo")
	if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	createTmpFile(t, filepath, "sample data")

	opts := NewCopyOptions(genericclioptions.NewTestIOStreamsDiscard())

	writer := &bytes.Buffer{}
	if err := makeTar(dir, dir, writer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reader := bytes.NewBuffer(writer.Bytes())
	err = opts.untarAll(reader, dir2, "verylongprefix-showing-the-tar-was-tempered-with")
	if err == nil || !strings.Contains(err.Error(), "tar contents corrupted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCopyToLocalFileOrDir tests untarAll in two cases :
// 1: copy pod file to local file
// 2: copy pod file into local directory
func TestCopyToLocalFileOrDir(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "input")
	dir2, err2 := ioutil.TempDir(os.TempDir(), "output")
	if err != nil || err2 != nil {
		t.Errorf("unexpected error: %v | %v", err, err2)
		t.FailNow()
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
		if err := os.RemoveAll(dir2); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
	}()

	files := []struct {
		name          string
		data          string
		dest          string
		destDirExists bool
	}{
		{
			name:          "foo",
			data:          "foobarbaz",
			dest:          "path/to/dest",
			destDirExists: false,
		},
		{
			name:          "dir/blah",
			data:          "bazblahfoo",
			dest:          "dest/file/path",
			destDirExists: true,
		},
	}

	for _, file := range files {
		func() {
			// setup
			srcFilePath := filepath.Join(dir, file.name)
			destPath := filepath.Join(dir2, file.dest)
			if err := os.MkdirAll(filepath.Dir(srcFilePath), 0755); err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			createTmpFile(t, srcFilePath, file.data)
			if file.destDirExists {
				if err := os.MkdirAll(destPath, 0755); err != nil {
					t.Errorf("unexpected error: %v", err)
					t.FailNow()
				}
			}

			// start tests
			srcTarFilePath := filepath.Join(dir, file.name+".tar")
			// here use tar command to create tar file instead of calling makeTar func
			// because makeTar func can not generate correct header name
			err = exec.Command("tar", "cf", srcTarFilePath, srcFilePath).Run()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			srcTarFile, err := os.Open(srcTarFilePath)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			defer srcTarFile.Close()

			opts := NewCopyOptions(genericclioptions.NewTestIOStreamsDiscard())
			if err := opts.untarAll(srcTarFile, destPath, getPrefix(srcFilePath)); err != nil {
				t.Errorf("unexpected error: %v", err)
				t.FailNow()
			}
			actualDestFilePath := destPath
			if file.destDirExists {
				actualDestFilePath = filepath.Join(destPath, filepath.Base(srcFilePath))
			}
			_, err = os.Stat(actualDestFilePath)
			if err != nil && os.IsNotExist(err) {
				t.Errorf("expecting %s exists, but actually it's missing", actualDestFilePath)
			}
			cmpFileData(t, actualDestFilePath, file.data)
		}()
	}

}

func TestTarDestinationName(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "input")
	dir2, err2 := ioutil.TempDir(os.TempDir(), "output")
	if err != nil || err2 != nil {
		t.Errorf("unexpected error: %v | %v", err, err2)
		t.FailNow()
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
		if err := os.RemoveAll(dir2); err != nil {
			t.Errorf("Unexpected error cleaning up: %v", err)
		}
	}()

	files := []struct {
		name string
		data string
	}{
		{
			name: "foo",
			data: "foobarbaz",
		},
		{
			name: "dir/blah",
			data: "bazblahfoo",
		},
		{
			name: "some/other/directory",
			data: "with more data here",
		},
		{
			name: "blah",
			data: "same file name different data",
		},
	}

	// ensure files exist on disk
	for _, file := range files {
		filepath := path.Join(dir, file.name)
		if err := os.MkdirAll(path.Dir(filepath), 0755); err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}
		createTmpFile(t, filepath, file.data)
	}

	reader, writer := io.Pipe()
	go func() {
		if err := makeTar(dir, dir2, writer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	tarReader := tar.NewReader(reader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Errorf("unexpected error: %v", err)
			t.FailNow()
		}

		if !strings.HasPrefix(hdr.Name, path.Base(dir2)) {
			t.Errorf("expected %q as destination filename prefix, saw: %q", path.Base(dir2), hdr.Name)
		}
	}
}

func TestBadTar(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "dest")
	if err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}
	defer os.RemoveAll(dir)

	// More or less cribbed from https://golang.org/pkg/archive/tar/#example__minimal
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		name string
		body string
	}{
		{"/prefix/../../../tmp/foo", "Up to temp"},
		{"/prefix/foo/bar/../../home/bburns/names.txt", "Down and back"},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.name,
			Mode: 0600,
			Size: int64(len(file.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Errorf("unexpected error: %v ", err)
			t.FailNow()
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Errorf("unexpected error: %v ", err)
			t.FailNow()
		}
	}
	if err := tw.Close(); err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}

	opts := NewCopyOptions(genericclioptions.NewTestIOStreamsDiscard())
	if err := opts.untarAll(&buf, dir, "/prefix"); err != nil {
		t.Errorf("unexpected error: %v ", err)
		t.FailNow()
	}

	for _, file := range files {
		_, err := os.Stat(path.Join(dir, path.Clean(file.name[len("/prefix"):])))
		if err != nil {
			t.Errorf("Error finding file: %v", err)
		}
	}

}

func TestClean(t *testing.T) {
	tests := []struct {
		input   string
		cleaned string
	}{
		{
			"../../../tmp/foo",
			"/tmp/foo",
		},
		{
			"/../../../tmp/foo",
			"/tmp/foo",
		},
	}

	for _, test := range tests {
		out := clean(test.input)
		if out != test.cleaned {
			t.Errorf("Expected: %s, saw %s", test.cleaned, out)
		}
	}
}

func TestCopyToPod(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("test")
	ns := scheme.Codecs
	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.Client = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			responsePod := &v1.Pod{}
			return &http.Response{StatusCode: http.StatusNotFound, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, responsePod))))}, nil
		}),
	}

	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()
	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdCp(tf, ioStreams)

	srcFile, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
	defer os.RemoveAll(srcFile)

	tests := map[string]struct {
		dest        string
		expectedErr bool
	}{
		"copy to directory": {
			dest:        "/tmp/",
			expectedErr: false,
		},
		"copy to root": {
			dest:        "/",
			expectedErr: false,
		},
		"copy to empty file name": {
			dest:        "",
			expectedErr: true,
		},
	}

	for name, test := range tests {
		opts := NewCopyOptions(ioStreams)
		src := fileSpec{
			File: srcFile,
		}
		dest := fileSpec{
			PodNamespace: "pod-ns",
			PodName:      "pod-name",
			File:         test.dest,
		}
		opts.Complete(tf, cmd)
		t.Run(name, func(t *testing.T) {
			err = opts.copyToPod(src, dest, &kexec.ExecOptions{})
			//If error is NotFound error , it indicates that the
			//request has been sent correctly.
			//Treat this as no error.
			if test.expectedErr && errors.IsNotFound(err) {
				t.Errorf("expected error but didn't get one")
			}
			if !test.expectedErr && !errors.IsNotFound(err) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCopyToPodNoPreserve(t *testing.T) {
	tf := cmdtesting.NewTestFactory().WithNamespace("test")
	ns := scheme.Codecs
	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	tf.Client = &fake.RESTClient{
		GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
		NegotiatedSerializer: ns,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			responsePod := &v1.Pod{}
			return &http.Response{StatusCode: http.StatusNotFound, Header: cmdtesting.DefaultHeader(), Body: ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, responsePod))))}, nil
		}),
	}

	tf.ClientConfigVal = cmdtesting.DefaultClientConfig()
	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()

	cmd := NewCmdCp(tf, ioStreams)

	srcFile, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
	defer os.RemoveAll(srcFile)

	tests := map[string]struct {
		expectedCmd []string
		nopreserve  bool
	}{
		"copy to pod no preserve user and permissions": {
			expectedCmd: []string{"tar", "--no-same-permissions", "--no-same-owner", "-xf", "-", "-C", "."},
			nopreserve:  true,
		},
		"copy to pod preserve user and permissions": {
			expectedCmd: []string{"tar", "-xf", "-", "-C", "."},
			nopreserve:  false,
		},
	}
	opts := NewCopyOptions(ioStreams)
	src := fileSpec{
		File: srcFile,
	}
	dest := fileSpec{
		PodNamespace: "pod-ns",
		PodName:      "pod-name",
		File:         "foo",
	}
	opts.Complete(tf, cmd)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			options := &kexec.ExecOptions{}
			opts.NoPreserve = test.nopreserve
			err = opts.copyToPod(src, dest, options)
			if !(reflect.DeepEqual(test.expectedCmd, options.Command)) {
				t.Errorf("expected cmd: %v, got: %v", test.expectedCmd, options.Command)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectedErr bool
	}{
		{
			name:        "Validate Succeed",
			args:        []string{"1", "2"},
			expectedErr: false,
		},
		{
			name:        "Validate Fail",
			args:        []string{"1", "2", "3"},
			expectedErr: true,
		},
	}
	tf := cmdtesting.NewTestFactory()
	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()
	opts := NewCopyOptions(ioStreams)
	cmd := NewCmdCp(tf, ioStreams)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := opts.Validate(cmd, test.args)
			if (err != nil) != test.expectedErr {
				t.Errorf("expected error: %v, saw: %v, error: %v", test.expectedErr, err != nil, err)
			}
		})
	}
}

func createTmpFile(t *testing.T, filepath, data string) {
	f, err := os.Create(filepath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, bytes.NewBuffer([]byte(data))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func cmpFileData(t *testing.T, filePath, data string) {
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer f.Close()
	buff := &bytes.Buffer{}
	if _, err := io.Copy(buff, f); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if data != string(buff.Bytes()) {
		t.Fatalf("expected: %s, saw: %s", data, string(buff.Bytes()))
	}
}
