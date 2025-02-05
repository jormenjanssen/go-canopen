package canopen

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/angelodlfrtr/go-can"
)

type ISDOClient interface {
	FindName(name string) DicObject
	Read(index uint16, subIndex uint8) ([]byte, error)
	Send(req []byte, expectFunc networkFramesChanFilterFunc, timeout *time.Duration, retryCount *int) (*can.Frame, error)
	SendRequest(req []byte) error
	Write(index uint16, subIndex uint8, forceSegment bool, data []byte) error
}

type SDOWriter struct {
	SDOClient    ISDOClient
	Index        uint16
	SubIndex     uint8
	Toggle       uint8
	Pos          int
	Size         uint32
	ForceSegment bool
}

func NewSDOWriter(sdoClient ISDOClient, index uint16, subIndex uint8, forceSegment bool) *SDOWriter {
	return &SDOWriter{
		SDOClient:    sdoClient,
		Index:        index,
		SubIndex:     subIndex,
		ForceSegment: forceSegment,
	}
}

// buildRequestDownloadBuf
func (writer *SDOWriter) buildRequestDownloadBuf(data []byte, size *uint32) (string, []byte) {
	buf := make([]byte, 8) // 8 len is important
	command := SDORequestDownload

	if size != nil {
		command |= SDOSizeSpecified
		binary.LittleEndian.PutUint32(buf[4:], *size)
	}

	// Write object index / subindex
	binary.LittleEndian.PutUint16(buf[1:], writer.Index)
	buf[3] = writer.SubIndex

	// Segmented download
	if size == nil || ((size != nil) && *size > 4) || writer.ForceSegment {
		buf[0] = command
		writer.Toggle = uint8(0x00)
		writer.Size = uint32(len(data))
		writer.Pos = 0
		return "segmented", buf
	}

	// Expedited download, so data is directly in download request message
	command = SDORequestDownload | SDOExpedited | SDOSizeSpecified
	command |= (4 - uint8(*size)) << 2
	buf[0] = command

	// Write data
	for i := 0; i < int(*size); i++ {
		buf[i+4] = data[i]
	}

	return "expedited", buf
}

// RequestDownload returns data if EXPEDITED, else nil
func (writer *SDOWriter) RequestDownload(data []byte) error {
	// Get data size
	var size uint32

	if data != nil {
		size = uint32(len(data))
	}

	downloadType, cmd := writer.buildRequestDownloadBuf(data, &size)
	if downloadType == "segmented" {
		return writer.writeBufferSegmented(cmd, data)
	}

	return writer.writeBufferExpedited(cmd)
}

func (writer *SDOWriter) writeBufferExpedited(cmd []byte) error {
	expectFunc := func(frm *can.Frame) bool {
		resCommand := frm.Data[0]
		resIndex := binary.LittleEndian.Uint16(frm.Data[1:])
		resSubindex := frm.Data[3]

		// Check response validity
		if (resCommand & 0xE0) != SDOResponseDownload {
			return false
		}

		if resIndex != writer.Index {
			return false
		}

		if resSubindex != writer.SubIndex {
			return false
		}

		return true
	}

	_, err := writer.SDOClient.Send(cmd, &expectFunc, nil, nil)
	return err
}

func (writer *SDOWriter) writeBufferSegmented(cmd []byte, data []byte) error {
	expectFunc := func(frm *can.Frame) bool {
		resCommand := frm.Data[0]
		resIndex := binary.LittleEndian.Uint16(frm.Data[1:])
		resSubindex := frm.Data[3]

		// Check response validity
		if (resCommand & 0xE0) != SDOResponseDownload {
			return false
		}

		if resIndex != writer.Index {
			return false
		}

		if resSubindex != writer.SubIndex {
			return false
		}

		return true
	}

	_, err := writer.SDOClient.Send(cmd, &expectFunc, nil, nil)

	if err != nil {
		return err
	}

	for {
		buf := make([]byte, 8)
		if writer.Pos >= int(writer.Size) {
			break
		}
		buf[0] = SDORequestSegmentDownload | writer.Toggle
		frameSize := min(int(writer.Size)-writer.Pos, 7)
		fmt.Println(frameSize)
		if writer.Pos+frameSize >= int(writer.Size)-1 {
			buf[0] = buf[0] | SDONoMoreData
		}
		buf[0] = buf[0] | (7-uint8(frameSize))<<1
		copy(buf[1:frameSize+1], data[writer.Pos:writer.Pos+frameSize])

		expectFunc := func(frm *can.Frame) bool {
			resCommand := frm.Data[0]
			// Check response validity
			if (resCommand & 0xE0) != SDOResponseSegmentDownload {
				return false
			}
			if (resCommand & SDOToggleBit) != writer.Toggle {
				return false
			}
			return true
		}
		_, err := writer.SDOClient.Send(buf, &expectFunc, nil, nil)
		if err != nil {
			return err
		}
		writer.Toggle = writer.Toggle ^ SDOToggleBit
		writer.Pos = writer.Pos + frameSize
	}
	return nil
}

// Write data to sdo client
func (writer *SDOWriter) Write(data []byte) error {
	return writer.RequestDownload(data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
