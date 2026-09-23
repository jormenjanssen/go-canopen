package canopen

import (
	"encoding/binary"
	"errors"
)

type SDOWriter struct {
	SDOClient    *SDOClient
	Index        uint16
	SubIndex     uint8
	Done         bool
	Toggle       uint8
	Pos          int
	Size         uint32
	ForceSegment bool
}

func NewSDOWriter(sdoClient *SDOClient, index uint16, subIndex uint8, forceSegment bool) *SDOWriter {
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

	downloadType, buf := writer.buildRequestDownloadBuf(data, &size)
	if downloadType == "segmented" {
		return errors.New("SDO segmented download not yet implemented")
	}

	expectFunc := writer.SDOClient.expectInitiateResponse(SDOResponseDownload, writer.Index, writer.SubIndex)

	_, err := writer.SDOClient.Send(buf, expectFunc, nil, nil)
	return err
}

// Write data to sdo client
func (writer *SDOWriter) Write(data []byte) error {
	return writer.RequestDownload(data)
}
