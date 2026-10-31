package main

import "time"

var version = "0.1.0"

type Category string

const (
	CategoryApplication Category = "Application"
	CategoryAppData     Category = "Application data"
	CategoryAppleMedia  Category = "Apple media"
	CategoryAppleBackup Category = "Apple device backup"
	CategoryDeveloper   Category = "Developer data"
	CategoryContainer   Category = "Container storage"
)

type ItemStatus string

const (
	StatusAvailable   ItemStatus = "available"
	StatusMoved       ItemStatus = "moved"
	StatusDiskOffline ItemStatus = "disk offline"
	StatusUnavailable ItemStatus = "unavailable"
)

type Item struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Category          Category   `json:"category"`
	SourcePath        string     `json:"source_path"`
	DestinationPath   string     `json:"destination_path,omitempty"`
	VolumeUUID        string     `json:"volume_uuid,omitempty"`
	SizeBytes         int64      `json:"size_bytes"`
	Status            ItemStatus `json:"status"`
	Reason            string     `json:"reason,omitempty"`
	Owner             string     `json:"owner,omitempty"`
	SourceFingerprint string     `json:"-"`
}

type VolumeRating string

const (
	RatingBest        VolumeRating = "Recommended"
	RatingGood        VolumeRating = "Suitable"
	RatingCaution     VolumeRating = "Caution"
	RatingUnavailable VolumeRating = "Unavailable"
)

type Volume struct {
	Name        string       `json:"name"`
	MountPath   string       `json:"mount_path"`
	UUID        string       `json:"uuid,omitempty"`
	Device      string       `json:"device,omitempty"`
	FileSystem  string       `json:"filesystem"`
	Bus         string       `json:"bus,omitempty"`
	SolidState  *bool        `json:"solid_state,omitempty"`
	FreeBytes   int64        `json:"free_bytes"`
	TotalBytes  int64        `json:"total_bytes"`
	Writable    bool         `json:"writable"`
	Network     bool         `json:"network"`
	Physical    bool         `json:"physical"`
	Rating      VolumeRating `json:"rating"`
	ReadMBps    float64      `json:"read_mbps,omitempty"`
	WriteMBps   float64      `json:"write_mbps,omitempty"`
	Benchmarked time.Time    `json:"benchmarked_at,omitempty"`
}

type Operation struct {
	ID                      string    `json:"id"`
	ItemID                  string    `json:"item_id"`
	Origin                  string    `json:"origin,omitempty"`
	Name                    string    `json:"name"`
	Category                Category  `json:"category"`
	SourcePath              string    `json:"source_path"`
	DestinationPath         string    `json:"destination_path"`
	PreviousDestinationPath string    `json:"previous_destination_path,omitempty"`
	VerificationSHA256      string    `json:"verification_sha256,omitempty"`
	VolumeUUID              string    `json:"volume_uuid"`
	VolumeMountPath         string    `json:"volume_mount_path,omitempty"`
	VolumeName              string    `json:"volume_name"`
	SizeBytes               int64     `json:"size_bytes"`
	Phase                   string    `json:"phase"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	StagingPath             string    `json:"staging_path,omitempty"`
	BackupPath              string    `json:"backup_path,omitempty"`
	Error                   string    `json:"error,omitempty"`
}

type DiskBenchmark struct {
	ReadMBps  float64   `json:"read_mbps"`
	WriteMBps float64   `json:"write_mbps"`
	At        time.Time `json:"at"`
}
