/*
Copyright 2017 The Kubernetes Authors.

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

package validation

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/admissionregistration"
)

func strPtr(s string) *string { return &s }

func int32Ptr(i int32) *int32 { return &i }

func newValidatingWebhookConfiguration(hooks []admissionregistration.Webhook, defaultAdmissionReviewVersions bool) *admissionregistration.ValidatingWebhookConfiguration {
	// If the test case did not specify an AdmissionReviewVersions, default it so the test passes as
	// this field will be defaulted in production code.
	for i := range hooks {
		if defaultAdmissionReviewVersions && len(hooks[i].AdmissionReviewVersions) == 0 {
			hooks[i].AdmissionReviewVersions = []string{"v1beta1"}
		}
	}
	return &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Webhooks: hooks,
	}
}

// TODO: Add TestValidateMutatingWebhookConfiguration to test validation for mutating webhooks.

func TestValidateValidatingWebhookConfiguration(t *testing.T) {
	validClientConfig := admissionregistration.WebhookClientConfig{
		URL: strPtr("https://example.com"),
	}
	tests := []struct {
		name          string
		config        *admissionregistration.ValidatingWebhookConfiguration
		expectedError string
	}{
		{
			name: "should fail on bad AdmissionReviewVersion value",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"0v"},
				},
			}, true),
			expectedError: `Invalid value: "0v": a DNS-1035 label`,
		},
		{
			name: "should pass on valid AdmissionReviewVersion",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"v1beta1"},
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "should pass on mix of accepted and unaccepted AdmissionReviewVersion",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"v1beta1", "invalid-version"},
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "should fail on invalid AdmissionReviewVersion",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"invalidVersion"},
				},
			}, true),
			expectedError: `Invalid value: []string{"invalidVersion"}`,
		},
		{
			name: "should fail on duplicate AdmissionReviewVersion",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"v1beta1", "v1beta1"},
				},
			}, true),
			expectedError: `Invalid value: "v1beta1": duplicate version`,
		},
		{
			name: "all Webhooks must have a fully qualified name",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
				},
				{
					Name:         "k8s.io",
					ClientConfig: validClientConfig,
				},
				{
					Name:         "",
					ClientConfig: validClientConfig,
				},
			}, true),
			expectedError: `webhooks[1].name: Invalid value: "k8s.io": should be a domain with at least three segments separated by dots, webhooks[2].name: Required value`,
		},
		{
			name: "Operations must not be empty or nil",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a"},
							},
						},
						{
							Operations: nil,
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a"},
							},
						},
					},
				},
			}, true),
			expectedError: `webhooks[0].rules[0].operations: Required value, webhooks[0].rules[1].operations: Required value`,
		},
		{
			name: "\"\" is NOT a valid operation",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE", ""},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a"},
							},
						},
					},
				},
			}, true),
			expectedError: `Unsupported value: ""`,
		},
		{
			name: "operation must be either create/update/delete/connect",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"PATCH"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a"},
							},
						},
					},
				},
			}, true),
			expectedError: `Unsupported value: "PATCH"`,
		},
		{
			name: "wildcard operation cannot be mixed with other strings",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE", "*"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a"},
							},
						},
					},
				},
			}, true),
			expectedError: `if '*' is present, must not specify other operations`,
		},
		{
			name: `resource "*" can co-exist with resources that have subresources`,
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"*", "a/b", "a/*", "*/b"},
							},
						},
					},
				},
			}, true),
		},
		{
			name: `resource "*" cannot mix with resources that don't have subresources`,
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"*", "a"},
							},
						},
					},
				},
			}, true),
			expectedError: `if '*' is present, must not specify other resources without subresources`,
		},
		{
			name: "resource a/* cannot mix with a/x",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a/*", "a/x"},
							},
						},
					},
				},
			}, true),
			expectedError: `webhooks[0].rules[0].resources[1]: Invalid value: "a/x": if 'a/*' is present, must not specify a/x`,
		},
		{
			name: "resource a/* can mix with a",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"a/*", "a"},
							},
						},
					},
				},
			}, true),
		},
		{
			name: "resource */a cannot mix with x/a",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"*/a", "x/a"},
							},
						},
					},
				},
			}, true),
			expectedError: `webhooks[0].rules[0].resources[1]: Invalid value: "x/a": if '*/a' is present, must not specify x/a`,
		},
		{
			name: "resource */* cannot mix with other resources",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					Rules: []admissionregistration.RuleWithOperations{
						{
							Operations: []admissionregistration.OperationType{"CREATE"},
							Rule: admissionregistration.Rule{
								APIGroups:   []string{"a"},
								APIVersions: []string{"a"},
								Resources:   []string{"*/*", "a"},
							},
						},
					},
				},
			}, true),
			expectedError: `webhooks[0].rules[0].resources: Invalid value: []string{"*/*", "a"}: if '*/*' is present, must not specify other resources`,
		},
		{
			name: "FailurePolicy can only be \"Ignore\" or \"Fail\"",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					FailurePolicy: func() *admissionregistration.FailurePolicyType {
						r := admissionregistration.FailurePolicyType("other")
						return &r
					}(),
				},
			}, true),
			expectedError: `webhooks[0].failurePolicy: Unsupported value: "other": supported values: "Fail", "Ignore"`,
		},
		{
			name: "SideEffects can only be \"Unknown\", \"None\", \"Some\", or \"NoneOnDryRun\"",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
					SideEffects: func() *admissionregistration.SideEffectClass {
						r := admissionregistration.SideEffectClass("other")
						return &r
					}(),
				},
			}, true),
			expectedError: `webhooks[0].sideEffects: Unsupported value: "other": supported values: "None", "NoneOnDryRun", "Some", "Unknown"`,
		},
		{
			name: "both service and URL missing",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{},
				},
			}, true),
			expectedError: `exactly one of`,
		},
		{
			name: "both service and URL provided",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
						},
						URL: strPtr("example.com/k8s/webhook"),
					},
				},
			}, true),
			expectedError: `[0].clientConfig: Required value: exactly one of url or service is required`,
		},
		{
			name: "blank URL",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr(""),
					},
				},
			}, true),
			expectedError: `[0].clientConfig.url: Invalid value: "": host must be provided`,
		},
		{
			name: "wrong scheme",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("http://example.com"),
					},
				},
			}, true),
			expectedError: `https`,
		},
		{
			name: "missing host",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("https:///fancy/webhook"),
					},
				},
			}, true),
			expectedError: `host must be provided`,
		},
		{
			name: "fragment",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("https://example.com/#bookmark"),
					},
				},
			}, true),
			expectedError: `"bookmark": fragments are not permitted`,
		},
		{
			name: "query",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("https://example.com?arg=value"),
					},
				},
			}, true),
			expectedError: `"arg=value": query parameters are not permitted`,
		},
		{
			name: "user",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("https://harry.potter@example.com/"),
					},
				},
			}, true),
			expectedError: `"harry.potter": user information is not permitted`,
		},
		{
			name: "just totally wrong",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						URL: strPtr("arg#backwards=thisis?html.index/port:host//:https"),
					},
				},
			}, true),
			expectedError: `host must be provided`,
		},
		{
			name: "path must start with slash",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("foo/"),
						},
					},
				},
			}, true),
			expectedError: `clientConfig.service.path: Invalid value: "foo/": must start with a '/'`,
		},
		{
			name: "path accepts slash",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("/"),
						},
					},
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "path accepts no trailing slash",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("/foo"),
						},
					},
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "path fails //",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("//"),
						},
					},
				},
			}, true),
			expectedError: `clientConfig.service.path: Invalid value: "//": segment[0] may not be empty`,
		},
		{
			name: "path no empty step",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("/foo//bar/"),
						},
					},
				},
			}, true),
			expectedError: `clientConfig.service.path: Invalid value: "/foo//bar/": segment[1] may not be empty`,
		}, {
			name: "path no empty step 2",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("/foo/bar//"),
						},
					},
				},
			}, true),
			expectedError: `clientConfig.service.path: Invalid value: "/foo/bar//": segment[2] may not be empty`,
		},
		{
			name: "path no non-subdomain",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name: "webhook.k8s.io",
					ClientConfig: admissionregistration.WebhookClientConfig{
						Service: &admissionregistration.ServiceReference{
							Namespace: "ns",
							Name:      "n",
							Path:      strPtr("/apis/foo.bar/v1alpha1/--bad"),
						},
					},
				},
			}, true),
			expectedError: `clientConfig.service.path: Invalid value: "/apis/foo.bar/v1alpha1/--bad": segment[3]: a DNS-1123 subdomain`,
		},
		{
			name: "timeout seconds cannot be greater than 30",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:           "webhook.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(31),
				},
			}, true),
			expectedError: `webhooks[0].timeoutSeconds: Invalid value: 31: the timeout value must be between 1 and 30 seconds`,
		},
		{
			name: "timeout seconds cannot be smaller than 1",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:           "webhook.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(0),
				},
			}, true),
			expectedError: `webhooks[0].timeoutSeconds: Invalid value: 0: the timeout value must be between 1 and 30 seconds`,
		},
		{
			name: "timeout seconds must be positive",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:           "webhook.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(-1),
				},
			}, true),
			expectedError: `webhooks[0].timeoutSeconds: Invalid value: -1: the timeout value must be between 1 and 30 seconds`,
		},
		{
			name: "valid timeout seconds",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:           "webhook.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(1),
				},
				{
					Name:           "webhook2.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(15),
				},
				{
					Name:           "webhook3.k8s.io",
					ClientConfig:   validClientConfig,
					TimeoutSeconds: int32Ptr(30),
				},
			}, true),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateValidatingWebhookConfiguration(test.config)
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("expected to contain %s, got %s", e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("unexpected no error, expected to contain %s", test.expectedError)
				}
			}
		})

	}
}

func TestValidateValidatingWebhookConfigurationUpdate(t *testing.T) {
	validClientConfig := admissionregistration.WebhookClientConfig{
		URL: strPtr("https://example.com"),
	}
	tests := []struct {
		name          string
		oldconfig     *admissionregistration.ValidatingWebhookConfiguration
		config        *admissionregistration.ValidatingWebhookConfiguration
		expectedError string
	}{
		{
			name: "should pass on valid new AdmissionReviewVersion",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"v1beta1"},
				},
			}, true),
			oldconfig: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "should pass on invalid AdmissionReviewVersion with invalid previous versions",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"invalid-v1", "invalid-v2"},
				},
			}, true),
			oldconfig: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"invalid-v0"},
				},
			}, true),
			expectedError: ``,
		},
		{
			name: "should fail on invalid AdmissionReviewVersion with valid previous versions",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"invalid-v1"},
				},
			}, true),
			oldconfig: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"v1beta1", "invalid-v1"},
				},
			}, true),
			expectedError: `Invalid value: []string{"invalid-v1"}`,
		},
		{
			name: "should fail on invalid AdmissionReviewVersion with missing previous versions",
			config: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:                    "webhook.k8s.io",
					ClientConfig:            validClientConfig,
					AdmissionReviewVersions: []string{"invalid-v1"},
				},
			}, true),
			oldconfig: newValidatingWebhookConfiguration([]admissionregistration.Webhook{
				{
					Name:         "webhook.k8s.io",
					ClientConfig: validClientConfig,
				},
			}, false),
			expectedError: `Invalid value: []string{"invalid-v1"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := ValidateValidatingWebhookConfigurationUpdate(test.config, test.oldconfig)
			err := errs.ToAggregate()
			if err != nil {
				if e, a := test.expectedError, err.Error(); !strings.Contains(a, e) || e == "" {
					t.Errorf("expected to contain %s, got %s", e, a)
				}
			} else {
				if test.expectedError != "" {
					t.Errorf("unexpected no error, expected to contain %s", test.expectedError)
				}
			}
		})

	}
}
