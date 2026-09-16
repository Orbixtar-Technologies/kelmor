package store

import "fmt"

func jobOperationID(jobID string, fence int64) string {
	return fmt.Sprintf("%s:%d", jobID, fence)
}

func resourceFenceKey(j *Job) string {
	if j == nil {
		return ""
	}
	if j.ResourceType == "" && j.ResourceID == "" {
		return "job/" + j.ID
	}
	return j.ResourceType + "/" + j.ResourceID
}

func acceptJobMutation(existing, incoming *Job) error {
	if existing == nil || incoming == nil {
		return nil
	}
	if existing.Fence == 0 {
		return nil
	}
	if incoming.Fence != existing.Fence {
		return ErrStaleFence
	}
	return nil
}
