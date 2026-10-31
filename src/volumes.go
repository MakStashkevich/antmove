package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const benchmarkBytes = 256 * 1024 * 1024

func scanVolumes() ([]Volume, error) {
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return nil, err
	}
	mountOutput, _ := commandOutput("/sbin/mount")
	benchmarks, _ := loadBenchmarks()
	var volumes []Volume
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		mountPath := filepath.Join("/Volumes", entry.Name())
		info, err := diskInfo(mountPath)
		if err != nil {
			continue
		}
		internal, internalKnown := boolProperty(info, "Internal")
		image, _ := boolProperty(info, "DiskImage")
		if (internalKnown && internal) || image {
			continue
		}
		device := stringProperty(info, "DeviceIdentifier")
		if !strings.HasPrefix(device, "disk") {
			continue
		}
		readOnly, _ := boolProperty(info, "ReadOnlyMedia")
		writableInfo, writableKnown := boolProperty(info, "Writable")
		writable := !readOnly && (!writableKnown || writableInfo)
		fsName := stringProperty(info, "FilesystemType", "FileSystemPersonality")
		if fsName == "" {
			fsName = "Unknown"
		}
		network := isNetworkMount(string(mountOutput), mountPath)
		free := int64Property(info, "FreeSpace")
		total := int64Property(info, "TotalSize")
		var stats unix.Statfs_t
		if unix.Statfs(mountPath, &stats) == nil {
			free = int64(stats.Bavail * uint64(stats.Bsize))
			if total == 0 {
				total = int64(stats.Blocks * uint64(stats.Bsize))
			}
		}
		var solidState *bool
		if value, ok := boolProperty(info, "SolidState"); ok {
			solidState = &value
		}
		volume := Volume{
			Name: stringProperty(info, "VolumeName"), MountPath: mountPath,
			UUID: stringProperty(info, "VolumeUUID"), Device: device,
			FileSystem: fsName, Bus: stringProperty(info, "BusProtocol"),
			SolidState: solidState, FreeBytes: free, TotalBytes: total,
			Writable: writable, Network: network,
			Physical: internalKnown && !internal && !image,
		}
		if volume.Name == "" {
			volume.Name = entry.Name()
		}
		volume.Rating = rateVolume(volume)
		if record, ok := benchmarks[volumeKey(volume)]; ok {
			volume.ReadMBps, volume.WriteMBps, volume.Benchmarked = record.ReadMBps, record.WriteMBps, record.At
		}
		volumes = append(volumes, volume)
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })
	return volumes, nil
}

func diskInfo(path string) (map[string]any, error) {
	plist, err := commandOutput("/usr/sbin/diskutil", "info", "-plist", path)
	if err != nil {
		return nil, err
	}
	return plistMap(plist)
}

func rateVolume(v Volume) VolumeRating {
	if !v.Writable || v.Network || !v.Physical {
		return RatingUnavailable
	}
	switch strings.ToLower(v.FileSystem) {
	case "apfs":
		if v.SolidState == nil || !*v.SolidState {
			return RatingGood
		}
		return RatingBest
	case "hfs", "hfs+", "journaled hfs+":
		return RatingGood
	case "exfat", "msdos", "fat32":
		return RatingCaution
	default:
		return RatingCaution
	}
}

func isNetworkMount(mountOutput, path string) bool {
	needle := " on " + strings.ReplaceAll(path, " ", "\\040") + " ("
	for _, line := range strings.Split(mountOutput, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		return strings.Contains(line, "smbfs") || strings.Contains(line, "nfs") ||
			strings.Contains(line, "afpfs") || strings.Contains(line, "webdav")
	}
	return false
}

func gameSuitability(v Volume) string {
	if v.ReadMBps <= 0 {
		return "Run am disks --test to measure"
	}
	if v.ReadMBps >= 700 {
		return "Excellent for games"
	}
	if v.ReadMBps >= 350 {
		return "Good for most games"
	}
	if v.ReadMBps >= 150 {
		return "Use caution with demanding games"
	}
	return "Not recommended for games"
}

func appSuitability(v Volume) string {
	if v.ReadMBps <= 0 {
		return "Run am disks --test to measure"
	}
	if v.ReadMBps >= 150 {
		return "Excellent for applications"
	}
	if v.ReadMBps >= 75 {
		return "Good for applications"
	}
	if v.ReadMBps >= 30 {
		return "Suitable for light applications"
	}
	return "May feel slow"
}

func messengerSuitability(v Volume) string {
	if v.ReadMBps <= 0 {
		return "Run am disks --test to measure"
	}
	if v.ReadMBps >= 100 {
		return "Excellent for messengers"
	}
	if v.ReadMBps >= 50 {
		return "Good for messengers"
	}
	if v.ReadMBps >= 20 {
		return "Suitable for lightweight messengers"
	}
	return "May feel slow"
}

func testVolume(path string) (DiskBenchmark, error) {
	volumes, err := scanVolumes()
	if err != nil {
		return DiskBenchmark{}, err
	}
	var selected *Volume
	for i := range volumes {
		if filepath.Clean(volumes[i].MountPath) == filepath.Clean(path) {
			selected = &volumes[i]
			break
		}
	}
	if selected == nil || selected.Rating == RatingUnavailable {
		return DiskBenchmark{}, fmt.Errorf("%q is not an available physical external disk", path)
	}
	if selected.FreeBytes > 0 && selected.FreeBytes < int64(benchmarkBytes)*2 {
		return DiskBenchmark{}, fmt.Errorf("disk needs at least 512 MiB free for a speed test")
	}
	if err := writableProbe(path); err != nil {
		return DiskBenchmark{}, fmt.Errorf("disk is not writable: %w", err)
	}
	measurement, err := measureDisk(path, benchmarkBytes)
	if err != nil {
		return DiskBenchmark{}, err
	}
	measurement.At = time.Now()
	benchmarks, _ := loadBenchmarks()
	benchmarks[volumeKey(*selected)] = measurement
	if err := saveBenchmarks(benchmarks); err != nil {
		return measurement, fmt.Errorf("measured speed, but could not save result: %w", err)
	}
	return measurement, nil
}

func volumeKey(volume Volume) string {
	if volume.UUID != "" {
		return volume.UUID
	}
	return volume.Device
}

func measureDisk(directory string, bytes int) (DiskBenchmark, error) {
	tmp, err := os.CreateTemp(directory, ".antmove-speed-*")
	if err != nil {
		return DiskBenchmark{}, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := unix.FcntlInt(tmp.Fd(), unix.F_NOCACHE, 1); err != nil {
		_ = tmp.Close()
		return DiskBenchmark{}, fmt.Errorf("could not disable disk cache: %w", err)
	}
	buffer := make([]byte, 4*1024*1024)
	if _, err := rand.Read(buffer); err != nil {
		_ = tmp.Close()
		return DiskBenchmark{}, err
	}
	writeStart := time.Now()
	remaining := bytes
	for remaining > 0 {
		chunk := len(buffer)
		if chunk > remaining {
			chunk = remaining
		}
		written, err := tmp.Write(buffer[:chunk])
		if err != nil {
			_ = tmp.Close()
			return DiskBenchmark{}, err
		}
		remaining -= written
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return DiskBenchmark{}, err
	}
	writeSeconds := time.Since(writeStart).Seconds()
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return DiskBenchmark{}, err
	}
	_, _ = unix.FcntlInt(tmp.Fd(), unix.F_NOCACHE, 1)
	readBuffer := make([]byte, len(buffer))
	readStart := time.Now()
	var read int64
	for read < int64(bytes) {
		count := int64(len(readBuffer))
		if count > int64(bytes)-read {
			count = int64(bytes) - read
		}
		n, err := tmp.Read(readBuffer[:count])
		read += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = tmp.Close()
			return DiskBenchmark{}, err
		}
	}
	readSeconds := time.Since(readStart).Seconds()
	if err := tmp.Close(); err != nil {
		return DiskBenchmark{}, err
	}
	mb := float64(bytes) / 1_000_000
	return DiskBenchmark{WriteMBps: mb / maxFloat(writeSeconds, 0.001), ReadMBps: mb / maxFloat(readSeconds, 0.001)}, nil
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func writableProbe(path string) error {
	f, err := os.CreateTemp(path, ".antmove-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func loadBenchmarks() (map[string]DiskBenchmark, error) {
	path := filepath.Join(stateDirectory(), "benchmarks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]DiskBenchmark{}, nil
	}
	var result map[string]DiskBenchmark
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]DiskBenchmark{}, err
	}
	return result, nil
}

func saveBenchmarks(records map[string]DiskBenchmark) error {
	return atomicJSON(filepath.Join(stateDirectory(), "benchmarks.json"), records)
}

func volumeMessage(v Volume) string {
	solid := "unknown"
	if v.SolidState != nil {
		if *v.SolidState {
			solid = "SSD"
		} else {
			solid = "HDD"
		}
	}
	speed := "not tested"
	if v.ReadMBps > 0 {
		speed = fmt.Sprintf("read %.0f / write %.0f MB/s", v.ReadMBps, v.WriteMBps)
	}
	bus := v.Bus
	if bus == "" {
		bus = "external"
	}
	return fmt.Sprintf("%s · %s %s · %s · %s · %s · free %s", v.Name, v.FileSystem, solid, bus,
		v.Rating, speed, formatBytes(v.FreeBytes))
}
