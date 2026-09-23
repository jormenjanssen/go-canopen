package canopen

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/angelodlfrtr/go-can"
)

const (
	fakeSDONodeID = 0x20

	// fakeSDOResponseDelay lets SDOClient.Send start waiting before the answer arrives,
	// TestSDOAbortWithImmediateResponse covers an answer that is already there before Send waits
	fakeSDOResponseDelay = 5 * time.Millisecond

	// sdoAbortMaxDuration is well below the 500 ms first retry timeout of SDOClient.Send
	sdoAbortMaxDuration = 100 * time.Millisecond
)

// fakeSDOTransport is an in memory can.Transport emulating an SDO server.
// Every written frame is passed to respond, the returned frames are read back in order.
// Without responseDelay the answer is queued before Write returns.
type fakeSDOTransport struct {
	mu            sync.Mutex
	rx            chan *can.Frame
	requests      []can.Frame
	responseDelay time.Duration
	respond       func(req can.Frame) []can.Frame
}

func (transport *fakeSDOTransport) Open() error {
	return nil
}

func (transport *fakeSDOTransport) Close() error {
	return nil
}

func (transport *fakeSDOTransport) ReadChan() chan *can.Frame {
	return transport.rx
}

func (transport *fakeSDOTransport) Write(frm *can.Frame) error {
	transport.mu.Lock()
	transport.requests = append(transport.requests, *frm)
	transport.mu.Unlock()

	responses := transport.respond(*frm)

	if transport.responseDelay == 0 {
		for i := range responses {
			transport.rx <- &responses[i]
		}

		return nil
	}

	go func() {
		time.Sleep(transport.responseDelay)

		for i := range responses {
			transport.rx <- &responses[i]
		}
	}()

	return nil
}

// sentRequests returns a copy of every frame written by the client
func (transport *fakeSDOTransport) sentRequests() []can.Frame {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	return append([]can.Frame{}, transport.requests...)
}

// newFakeSDONode runs a network on a fake transport answering after fakeSDOResponseDelay
// and adds node fakeSDONodeID to it
func newFakeSDONode(t *testing.T, respond func(req can.Frame) []can.Frame) (*fakeSDOTransport, *Node) {
	t.Helper()

	return newFakeSDONodeWithDelay(t, fakeSDOResponseDelay, respond)
}

// newFakeSDONodeWithDelay runs a network on a fake transport answering after responseDelay
// and adds node fakeSDONodeID to it
func newFakeSDONodeWithDelay(
	t *testing.T,
	responseDelay time.Duration,
	respond func(req can.Frame) []can.Frame,
) (*fakeSDOTransport, *Node) {
	t.Helper()

	transport := &fakeSDOTransport{
		rx:            make(chan *can.Frame, 64),
		responseDelay: responseDelay,
		respond:       respond,
	}

	network, err := NewNetwork(*can.NewBus(transport))
	if err != nil {
		t.Fatal(err)
	}

	if err := network.Run(); err != nil {
		t.Fatal(err)
	}

	node := network.AddNode(NewNode(fakeSDONodeID, nil, nil), NewDicObjectDic(), false)

	t.Cleanup(func() {
		if err := network.Stop(); err != nil {
			t.Error(err)
		}
	})

	return transport, node
}

// sdoServerFrame builds an initiate response or abort sent by the SDO server of nodeID
func sdoServerFrame(nodeID int, command uint8, index uint16, subIndex uint8, data [4]byte) can.Frame {
	frm := can.Frame{ArbitrationID: uint32(0x580 + nodeID), DLC: 8}
	frm.Data[0] = command
	binary.LittleEndian.PutUint16(frm.Data[1:], index)
	frm.Data[3] = subIndex
	copy(frm.Data[4:], data[:])

	return frm
}

func sdoAbortFrame(nodeID int, index uint16, subIndex uint8, code uint32) can.Frame {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], code)

	return sdoServerFrame(nodeID, SDOAbortTransfer, index, subIndex, data)
}

// sdoSegmentFrame builds an upload segment carrying up to 7 bytes
func sdoSegmentFrame(nodeID int, toggle uint8, data []byte, last bool) can.Frame {
	command := SDOResponseSegmentUpload | toggle | uint8(7-len(data))<<1
	if last {
		command |= SDONoMoreData
	}

	frm := can.Frame{ArbitrationID: uint32(0x580 + nodeID), DLC: 8}
	frm.Data[0] = command
	copy(frm.Data[1:], data)

	return frm
}

func requestObject(req can.Frame) (uint16, uint8) {
	return binary.LittleEndian.Uint16(req.Data[1:]), req.Data[3]
}

func assertSDOAbortError(t *testing.T, err error, index uint16, subIndex uint8, code uint32) {
	t.Helper()

	var abortErr *SDOAbortError
	if !errors.As(err, &abortErr) {
		t.Fatalf("expected *SDOAbortError, got %T: %v", err, err)
	}

	if abortErr.Index != index || abortErr.SubIndex != subIndex || abortErr.Code != code {
		t.Fatalf(
			"expected abort 0x%08X on 0x%04X:%02X, got 0x%08X on 0x%04X:%02X",
			code, index, subIndex, abortErr.Code, abortErr.Index, abortErr.SubIndex,
		)
	}
}

func assertSingleRequest(t *testing.T, transport *fakeSDOTransport) {
	t.Helper()

	if requests := transport.sentRequests(); len(requests) != 1 {
		t.Fatalf("expected 1 request without retries, got %d", len(requests))
	}
}

func TestSDOReadAbortReturnsSDOAbortError(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)
		return []can.Frame{sdoAbortFrame(fakeSDONodeID, index, subIndex, SDOAbortObjectDoesNotExist)}
	})

	start := time.Now()
	data, err := node.SDOClient.Read(0x2008, 1)
	elapsed := time.Since(start)

	if data != nil {
		t.Fatalf("expected no data, got %v", data)
	}

	assertSDOAbortError(t, err, 0x2008, 1, SDOAbortObjectDoesNotExist)

	if elapsed > sdoAbortMaxDuration {
		t.Fatalf("abort took %v, expected less than %v", elapsed, sdoAbortMaxDuration)
	}

	assertSingleRequest(t, transport)
}

func TestSDOWriteAbortReturnsSDOAbortError(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)
		return []can.Frame{sdoAbortFrame(fakeSDONodeID, index, subIndex, SDOAbortObjectDoesNotExist)}
	})

	start := time.Now()
	err := node.SDOClient.Write(0x2008, 1, false, []byte{0x01, 0x02, 0x03, 0x04})
	elapsed := time.Since(start)

	assertSDOAbortError(t, err, 0x2008, 1, SDOAbortObjectDoesNotExist)

	if elapsed > sdoAbortMaxDuration {
		t.Fatalf("abort took %v, expected less than %v", elapsed, sdoAbortMaxDuration)
	}

	assertSingleRequest(t, transport)
}

// TestSDOAbortWithImmediateResponse lets the server answer before SDOClient.Send waits for it,
// and stops the network right after the abort, before its listeners may have been scheduled
func TestSDOAbortWithImmediateResponse(t *testing.T) {
	respond := func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)
		return []can.Frame{sdoAbortFrame(fakeSDONodeID, index, subIndex, SDOAbortObjectDoesNotExist)}
	}

	operations := []struct {
		name string
		run  func(node *Node) error
	}{
		{
			name: "read",
			run: func(node *Node) error {
				_, err := node.SDOClient.Read(0x2008, 1)
				return err
			},
		},
		{
			name: "write",
			run: func(node *Node) error {
				return node.SDOClient.Write(0x2008, 1, false, []byte{0x01})
			},
		},
	}

	// Losing the answer and the teardown panic depend on goroutine scheduling,
	// repeating makes a regression of either show up reliably
	for i := range 20 {
		for _, op := range operations {
			passed := t.Run(fmt.Sprintf("%s %02d", op.name, i), func(t *testing.T) {
				transport, node := newFakeSDONodeWithDelay(t, 0, respond)

				start := time.Now()
				err := op.run(node)
				elapsed := time.Since(start)

				assertSDOAbortError(t, err, 0x2008, 1, SDOAbortObjectDoesNotExist)

				if elapsed > sdoAbortMaxDuration {
					t.Fatalf("abort took %v, expected less than %v", elapsed, sdoAbortMaxDuration)
				}

				assertSingleRequest(t, transport)
			})

			if !passed {
				return
			}
		}
	}
}

func TestSDOReadSegmentedAbortReturnsSDOAbortError(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)

		switch req.Data[0] & sdoCommandSpecifierMask {
		case SDORequestUpload:
			return []can.Frame{sdoServerFrame(fakeSDONodeID, 0x41, index, subIndex, [4]byte{14})}
		case SDORequestSegmentUpload:
			if req.Data[0]&SDOToggleBit == 0 {
				return []can.Frame{sdoSegmentFrame(fakeSDONodeID, 0, []byte("NIR-SEN"), false)}
			}

			// The abort for another object must be skipped, the one for 0x2008:01 ends the transfer
			return []can.Frame{
				sdoAbortFrame(fakeSDONodeID, 0x2009, 1, SDOAbortToggleBitNotAlternated),
				sdoAbortFrame(fakeSDONodeID, 0x2008, 1, SDOAbortGeneralError),
			}
		}

		return nil
	})

	start := time.Now()
	data, err := node.SDOClient.Read(0x2008, 1)
	elapsed := time.Since(start)

	if data != nil {
		t.Fatalf("expected no data, got %v", data)
	}

	assertSDOAbortError(t, err, 0x2008, 1, SDOAbortGeneralError)

	if elapsed > sdoAbortMaxDuration {
		t.Fatalf("abort took %v, expected less than %v", elapsed, sdoAbortMaxDuration)
	}

	if requests := transport.sentRequests(); len(requests) != 3 {
		t.Fatalf("expected initiate and 2 segment requests without retries, got %d", len(requests))
	}
}

func TestSDOReadExpedited(t *testing.T) {
	tests := []struct {
		name     string
		command  uint8
		expected []byte
	}{
		{name: "4 bytes", command: 0x43, expected: []byte{0x00, 0x00, 0xC8, 0x42}},
		{name: "2 bytes", command: 0x4B, expected: []byte{0x00, 0x00}},
		{name: "1 byte", command: 0x4F, expected: []byte{0x00}},
		{name: "size not specified", command: 0x42, expected: []byte{0x00, 0x00, 0xC8, 0x42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
				index, subIndex := requestObject(req)
				return []can.Frame{sdoServerFrame(fakeSDONodeID, tt.command, index, subIndex, [4]byte{0x00, 0x00, 0xC8, 0x42})}
			})

			data, err := node.SDOClient.Read(0x2007, 1)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(data, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, data)
			}

			assertSingleRequest(t, transport)

			req := transport.sentRequests()[0]
			index, subIndex := requestObject(req)

			if req.ArbitrationID != 0x600+fakeSDONodeID || req.Data[0] != SDORequestUpload || index != 0x2007 || subIndex != 1 {
				t.Fatalf("unexpected upload request %+v", req)
			}
		})
	}
}

func TestSDOReadSegmented(t *testing.T) {
	payload := []byte("NIR-SENSOR")
	offset := 0

	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)

		switch req.Data[0] & sdoCommandSpecifierMask {
		case SDORequestUpload:
			var size [4]byte
			binary.LittleEndian.PutUint32(size[:], uint32(len(payload)))

			return []can.Frame{sdoServerFrame(fakeSDONodeID, 0x41, index, subIndex, size)}
		case SDORequestSegmentUpload:
			end := min(offset+7, len(payload))
			segment := sdoSegmentFrame(fakeSDONodeID, req.Data[0]&SDOToggleBit, payload[offset:end], end == len(payload))
			offset = end

			return []can.Frame{segment}
		}

		return nil
	})

	data, err := node.SDOClient.Read(0x1008, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, payload) {
		t.Fatalf("expected %q, got %q", payload, data)
	}

	requests := transport.sentRequests()
	commands := []uint8{}

	for _, req := range requests {
		commands = append(commands, req.Data[0])
	}

	expectedCommands := []uint8{SDORequestUpload, SDORequestSegmentUpload, SDORequestSegmentUpload | SDOToggleBit}
	if !bytes.Equal(commands, expectedCommands) {
		t.Fatalf("expected request commands %#v, got %#v", expectedCommands, commands)
	}
}

func TestSDOWriteExpedited(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		index, subIndex := requestObject(req)
		return []can.Frame{sdoServerFrame(fakeSDONodeID, SDOResponseDownload, index, subIndex, [4]byte{})}
	})

	if err := node.SDOClient.Write(0x2001, 2, false, []byte{0x34, 0x12}); err != nil {
		t.Fatal(err)
	}

	assertSingleRequest(t, transport)

	req := transport.sentRequests()[0]
	expected := can.Frame{ArbitrationID: 0x600 + fakeSDONodeID, DLC: 8, Data: [8]byte{0x2B, 0x01, 0x20, 0x02, 0x34, 0x12}}

	if req != expected {
		t.Fatalf("expected download request %+v, got %+v", expected, req)
	}
}

func TestSDOReadIgnoresFramesForOtherNodeOrObject(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		// The client request looped back on the bus looks like an upload response for the same object
		echo := req

		return []can.Frame{
			echo,
			sdoAbortFrame(fakeSDONodeID+1, 0x2008, 1, SDOAbortObjectDoesNotExist),
			sdoAbortFrame(fakeSDONodeID, 0x2009, 1, SDOAbortObjectDoesNotExist),
			sdoAbortFrame(fakeSDONodeID, 0x2008, 2, SDOAbortSubIndexDoesNotExist),
			sdoServerFrame(fakeSDONodeID+1, 0x43, 0x2008, 1, [4]byte{0xEE, 0xEE, 0xEE, 0xEE}),
			sdoServerFrame(fakeSDONodeID, 0x43, 0x2009, 1, [4]byte{0xEE, 0xEE, 0xEE, 0xEE}),
			sdoServerFrame(fakeSDONodeID, 0x43, 0x2008, 1, [4]byte{0x01, 0x02, 0x03, 0x04}),
		}
	})

	data, err := node.SDOClient.Read(0x2008, 1)
	if err != nil {
		t.Fatal(err)
	}

	expected := []byte{0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(data, expected) {
		t.Fatalf("expected %v, got %v", expected, data)
	}

	assertSingleRequest(t, transport)
}

func TestSDOWriteIgnoresFramesForOtherNodeOrObject(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		return []can.Frame{
			sdoAbortFrame(fakeSDONodeID+1, 0x2008, 1, SDOAbortObjectDoesNotExist),
			sdoAbortFrame(fakeSDONodeID, 0x2009, 1, SDOAbortObjectDoesNotExist),
			sdoAbortFrame(fakeSDONodeID, 0x2008, 2, SDOAbortSubIndexDoesNotExist),
			sdoServerFrame(fakeSDONodeID+1, SDOResponseDownload, 0x2008, 1, [4]byte{}),
			sdoServerFrame(fakeSDONodeID, SDOResponseDownload, 0x2008, 1, [4]byte{}),
		}
	})

	if err := node.SDOClient.Write(0x2008, 1, false, []byte{0x01}); err != nil {
		t.Fatal(err)
	}

	assertSingleRequest(t, transport)
}

func TestSDOForeignAbortDoesNotStopRetries(t *testing.T) {
	transport, node := newFakeSDONode(t, func(req can.Frame) []can.Frame {
		return []can.Frame{
			sdoAbortFrame(fakeSDONodeID+1, 0x2008, 1, SDOAbortObjectDoesNotExist),
			sdoAbortFrame(fakeSDONodeID, 0x2009, 1, SDOAbortObjectDoesNotExist),
		}
	})

	reader := NewSDOReader(node.SDOClient, 0x2008, 1)
	timeout := 20 * time.Millisecond
	retryCount := 2

	frm, err := node.SDOClient.Send(
		reader.buildRequestUploadBuf(),
		node.SDOClient.expectInitiateResponse(SDOResponseUpload, 0x2008, 1),
		&timeout,
		&retryCount,
	)

	var abortErr *SDOAbortError
	if frm != nil || err == nil || errors.As(err, &abortErr) {
		t.Fatalf("expected a timeout, got frame %+v and error %v", frm, err)
	}

	if requests := transport.sentRequests(); len(requests) != retryCount {
		t.Fatalf("expected %d requests, got %d", retryCount, len(requests))
	}
}

func TestSDOResponseFilters(t *testing.T) {
	sdoClient := NewSDOClient(&Node{ID: fakeSDONodeID})
	reader := NewSDOReader(sdoClient, 0x2008, 1)
	otherNodeID := fakeSDONodeID + 1

	echo := sdoServerFrame(fakeSDONodeID, SDORequestUpload, 0x2008, 1, [4]byte{})
	echo.ArbitrationID = sdoClient.RXCobID

	// Which filters accept the frame
	type filterCase struct {
		name     string
		frm      *can.Frame
		upload   bool
		download bool
		segment  bool
	}

	tests := []filterCase{
		{name: "nil frame"},
		{
			name:   "upload response",
			frm:    framePtr(sdoServerFrame(fakeSDONodeID, 0x43, 0x2008, 1, [4]byte{})),
			upload: true,
		},
		{
			name:     "download response",
			frm:      framePtr(sdoServerFrame(fakeSDONodeID, SDOResponseDownload, 0x2008, 1, [4]byte{})),
			download: true,
		},
		{
			name:    "upload segment",
			frm:     framePtr(sdoSegmentFrame(fakeSDONodeID, 0, []byte{0x01}, true)),
			segment: true,
		},
		{
			name:     "abort",
			frm:      framePtr(sdoAbortFrame(fakeSDONodeID, 0x2008, 1, SDOAbortObjectDoesNotExist)),
			upload:   true,
			download: true,
			segment:  true,
		},
		{name: "abort from other node", frm: framePtr(sdoAbortFrame(otherNodeID, 0x2008, 1, SDOAbortGeneralError))},
		{name: "abort for other index", frm: framePtr(sdoAbortFrame(fakeSDONodeID, 0x2009, 1, SDOAbortGeneralError))},
		{name: "abort for other subindex", frm: framePtr(sdoAbortFrame(fakeSDONodeID, 0x2008, 2, SDOAbortGeneralError))},
		{name: "upload response from other node", frm: framePtr(sdoServerFrame(otherNodeID, 0x43, 0x2008, 1, [4]byte{}))},
		{name: "upload response for other index", frm: framePtr(sdoServerFrame(fakeSDONodeID, 0x43, 0x2009, 1, [4]byte{}))},
		{name: "upload segment from other node", frm: framePtr(sdoSegmentFrame(otherNodeID, 0, []byte{0x01}, true))},
		{name: "request echo on client cob id", frm: &echo},
	}

	filters := []struct {
		name     string
		filter   networkFramesChanFilterFunc
		accepted func(tt filterCase) bool
	}{
		{
			name:     "upload",
			filter:   sdoClient.expectInitiateResponse(SDOResponseUpload, 0x2008, 1),
			accepted: func(tt filterCase) bool { return tt.upload },
		},
		{
			name:     "download",
			filter:   sdoClient.expectInitiateResponse(SDOResponseDownload, 0x2008, 1),
			accepted: func(tt filterCase) bool { return tt.download },
		},
		{
			name:     "segment upload",
			filter:   reader.expectSegmentUploadResponse(),
			accepted: func(tt filterCase) bool { return tt.segment },
		},
	}

	for _, tt := range tests {
		for _, f := range filters {
			t.Run(f.name+"/"+tt.name, func(t *testing.T) {
				if got, expected := (*f.filter)(tt.frm), f.accepted(tt); got != expected {
					t.Fatalf("expected %v, got %v", expected, got)
				}
			})
		}
	}
}

func TestSDOAbortErrorMessage(t *testing.T) {
	tests := []struct {
		err      SDOAbortError
		expected string
	}{
		{
			err:      SDOAbortError{Index: 0x2008, SubIndex: 1, Code: SDOAbortObjectDoesNotExist},
			expected: "sdo abort 0x06020000 on index 0x2008 subindex 0x01: object does not exist in the object dictionary",
		},
		{
			err:      SDOAbortError{Index: 0x2010, SubIndex: 5, Code: SDOAbortSubIndexDoesNotExist},
			expected: "sdo abort 0x06090011 on index 0x2010 subindex 0x05: subindex does not exist",
		},
		{
			err:      SDOAbortError{Index: 0x1000, SubIndex: 0, Code: SDOAbortTimeout},
			expected: "sdo abort 0x05040000 on index 0x1000 subindex 0x00: SDO protocol timed out",
		},
		{
			err:      SDOAbortError{Index: 0x2000, SubIndex: 0xFF, Code: 0x12345678},
			expected: "sdo abort 0x12345678 on index 0x2000 subindex 0xFF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func framePtr(frm can.Frame) *can.Frame {
	return &frm
}
