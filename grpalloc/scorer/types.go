package scorer

// ResourceScoreFunc is a function which takes in (allocatable, usedByPod, usedByNode, requested, initContainer) resources
// and returns (resourceFits, score, usedByContainer, newUsedByPod, newUsedByNode)
type ResourceScoreFunc func(alloctable int64, usedByPod int64, usedByNode int64, requested []int64, initContainer bool) (bool, float64, int64, int64, int64)
