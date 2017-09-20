package scorer

import (
	"math"

	v1 "k8s.io/kubernetes/pkg/api"
)

// LeftoverScoreFunc provides default scoring function
func LeftoverScoreFunc(allocatable int64, usedByPod int64, usedByNode int64, requested []int64, initContainer bool) (
	found bool, score float64, usedByContainer int64, newUsedByPod, newUsedByNode int64) {

	totalRequested := int64(0)
	if requested != nil {
		for _, request := range requested {
			totalRequested += request
		}
	}

	usedByContainer = totalRequested

	if !initContainer {
		newUsedByPod = usedByPod + totalRequested
	} else {
		// for InitContainers
		if totalRequested > usedByPod {
			newUsedByPod = totalRequested
		} else {
			newUsedByPod = usedByPod
		}
	}
	newUsedByNode = usedByNode + (newUsedByPod - usedByPod)

	leftoverI := allocatable - newUsedByNode // >= 0 (between -Inf and allocatable if not found)
	leftoverF := float64(leftoverI)
	allocatableF := float64(allocatable)
	if allocatable != 0 {
		score = 1.0 - (leftoverF / allocatableF) // between 0.0 and 1.0 if leftover between 0 and allocatable
	} else {
		score = 0.0
	}
	found = (leftoverI >= 0)

	return // score will be between 0.0 and 1.0 if found = true
}

// AlwaysFoundScoreFunc provides something that always returns true
// want to make allocatable-used as close to requested
func AlwaysFoundScoreFunc(allocatable int64, usedByPod int64, usedByNode int64, requested []int64, initContainer bool) (
	found bool, score float64, usedByContainer int64, newUsedByPod, newUsedByNode int64) {

	found, score, usedByContainer, newUsedByPod, newUsedByNode = LeftoverScoreFunc(allocatable, usedByPod, usedByNode, requested, initContainer)
	diff := 1.0 - score          // between -Inf and 1.0
	diff = math.Max(-1.0, diff)  // between -1.0 and 1.0
	score = 1.0 - math.Abs(diff) // between 0.0 and 1.0
	found = true
	return
}

// Straight and simple C to Go translation from https://en.wikipedia.org/wiki/Hamming_weight
func popcount(x uint64) int {
	const (
		m1  = 0x5555555555555555 //binary: 0101...
		m2  = 0x3333333333333333 //binary: 00110011..
		m4  = 0x0f0f0f0f0f0f0f0f //binary:  4 zeros,  4 ones ...
		h01 = 0x0101010101010101 //the sum of 256 to the power of 0,1,2,3...
	)
	x -= (x >> 1) & m1             //put count of each 2 bits into those 2 bits
	x = (x & m2) + ((x >> 2) & m2) //put count of each 4 bits into those 4 bits
	x = (x + (x >> 4)) & m4        //put count of each 8 bits into those 8 bits
	return int((x * h01) >> 56)    //returns left 8 bits of x + (x<<8) + (x<<16) + (x<<24) + ...
}

// EnumScoreFunc returns bitwise score
func EnumScoreFunc(allocatable int64, usedByPod int64, usedByNode int64, requested []int64, initContainer bool) (
	found bool, score float64, usedByContainer int64, newUsedByPod, newUsedByNode int64) {

	totalRequested := int64(0)
	if requested != nil {
		for _, request := range requested {
			totalRequested |= request
		}
	}

	usedMask := uint64(allocatable & (usedByPod | totalRequested))
	bitCntAlloc := popcount(uint64(allocatable))
	bitCntUsed := popcount(usedMask)
	leftoverI := bitCntAlloc - bitCntUsed
	leftoverF := float64(leftoverI)
	allocatableF := float64(bitCntAlloc)
	if bitCntAlloc != 0 {
		score = 1.0 - (leftoverF / allocatableF)
	} else {
		score = 0.0
	}
	if totalRequested != 0 {
		found = ((uint64(allocatable) & uint64(totalRequested)) != 0) // at least one bit true
	} else {
		found = true
	}
	usedByContainer = totalRequested
	newUsedByPod = int64(usedMask)
	newUsedByNode = 0

	return
}

// DefaultScorer returns default scorer given a name
func DefaultScorer(resource v1.ResourceName) ResourceScoreFunc {
	if !PrecheckedResource(resource) {
		if !v1.IsEnumResource(resource) {
			return LeftoverScoreFunc
		}
		return EnumScoreFunc
	}
	return nil
}

func SetScorer(resource v1.ResourceName, scorerType int32) ResourceScoreFunc {
	if scorerType == v1.DefaultScorer {
		return DefaultScorer(resource)
	}
	if scorerType == v1.LeftOverScorer {
		return LeftoverScoreFunc
	}
	if scorerType == v1.EnumLeftOverScorer {
		return EnumScoreFunc
	}
	return nil
}

func PrecheckedResource(constraint v1.ResourceName) bool {
	return !v1.IsGroupResourceName(constraint)
}
