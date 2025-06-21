package proxmoxclient

import (
	"encoding/json"
	"time"
)

// VMID represents a Proxmox virtual machine or container ID
type VMID int

// TaskID represents a Proxmox task identifier
type TaskID string

// backupTaskResponse represents the response from creating a backup task
type backupTaskResponse struct {
	Data string `json:"data"`
}

// Resource represents a virtual machine or container in Proxmox.
type Resource struct {
	VMID     VMID    `json:"vmid"`
	Name     string  `json:"name"`
	Node     string  `json:"node"`
	Status   string  `json:"status"`
	Template int     `json:"template"`
	Type     string  `json:"type"`
	MaxMem   int64   `json:"maxmem"`
	MaxDisk  int64   `json:"maxdisk"`
	CPU      float64 `json:"cpu"`
	Mem      int64   `json:"mem"`
	Uptime   int64   `json:"uptime"`
}

// Storage represents a storage in Proxmox.
type Storage struct {
	Storage      string  `json:"storage"`
	Type         string  `json:"type"`
	Content      string  `json:"content"`
	Shared       int     `json:"shared"`
	Active       int     `json:"active"`
	Enabled      int     `json:"enabled"`
	Used         int64   `json:"used"`
	Available    int64   `json:"avail"`
	Total        int64   `json:"total"`
	UsedFraction float64 `json:"used_fraction"`
}

// Backup represents a backup in Proxmox storage.
type Backup struct {
	Content      string    `json:"content"`
	Format       string    `json:"format"`
	Size         int64     `json:"size"`
	CTime        time.Time `json:"ctime"`
	VolID        string    `json:"volid"`
	VMID         VMID      `json:"vmid"`
	Parent       string    `json:"parent,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	Protected    bool      `json:"protected,omitempty"`
	Verification string    `json:"verification,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for Backup to handle Unix timestamp conversion
func (b *Backup) UnmarshalJSON(data []byte) error {
	// Create a temporary struct to unmarshal the raw JSON
	type backupAlias Backup
	temp := struct {
		*backupAlias
		CTime int64 `json:"ctime"`
	}{
		backupAlias: (*backupAlias)(b),
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Convert Unix timestamp to time.Time
	b.CTime = time.Unix(temp.CTime, 0)

	return nil
}
