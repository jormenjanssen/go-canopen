package canopen

import (
	"errors"
	"time"

	"github.com/angelodlfrtr/go-can"
)

const (
	SDORequestUpload    uint8 = 2 << 5
	SDOResponseUpload   uint8 = 2 << 5
	SDORequestDownload  uint8 = 1 << 5
	SDOResponseDownload uint8 = 3 << 5

	SDORequestSegmentUpload    uint8 = 3 << 5
	SDOResponseSegmentUpload   uint8 = 0 << 5
	SDORequestSegmentDownload  uint8 = 0 << 5
	SDOResponseSegmentDownload uint8 = 1 << 5

	// SDOAbortTransfer is the command specifier of an abort transfer frame, sent by client or server
	SDOAbortTransfer uint8 = 4 << 5

	SDOExpedited     uint8 = 0x2
	SDOSizeSpecified uint8 = 0x1
	SDOToggleBit     uint8 = 0x10
	SDONoMoreData    uint8 = 0x1

	// sdoCommandSpecifierMask selects the command specifier bits of the first SDO frame byte
	sdoCommandSpecifierMask uint8 = 0xE0
)

// SDOClient represent an SDO client
type SDOClient struct {
	Node      *Node
	RXCobID   uint32
	TXCobID   uint32
	SendQueue []string
}

func NewSDOClient(node *Node) *SDOClient {
	return &SDOClient{
		Node:      node,
		RXCobID:   uint32(0x600 + node.ID),
		TXCobID:   uint32(0x580 + node.ID),
		SendQueue: []string{},
	}
}

// SendRequest to network bus
func (sdoClient *SDOClient) SendRequest(req []byte) error {
	return sdoClient.Node.Network.Send(sdoClient.RXCobID, req)
}

// FindName find an sdo object from object dictionary by name
func (sdoClient *SDOClient) FindName(name string) DicObject {
	if ob := sdoClient.Node.ObjectDic.FindName(name); ob != nil {
		ob.SetSDO(sdoClient)
		return ob
	}

	return nil
}

// expectInitiateResponse returns a filter accepting frames sent by the SDO server of this client
// with the given response command specifier, or an abort transfer, for index and subIndex
func (sdoClient *SDOClient) expectInitiateResponse(
	responseCommand uint8,
	index uint16,
	subIndex uint8,
) networkFramesChanFilterFunc {
	expectFunc := func(frm *can.Frame) bool {
		if frm == nil || frm.ArbitrationID != sdoClient.TXCobID {
			return false
		}

		resCommand := frm.Data[0] & sdoCommandSpecifierMask
		if resCommand != responseCommand && resCommand != SDOAbortTransfer {
			return false
		}

		return sdoFrameMatchesObject(frm, index, subIndex)
	}

	return &expectFunc
}

// Send message and optionaly wait for response.
// When the expected frame is an SDO abort transfer, retrying stops and an *SDOAbortError is returned.
func (sdoClient *SDOClient) Send(
	req []byte,
	expectFunc networkFramesChanFilterFunc,
	timeout *time.Duration,
	retryCount *int,
) (*can.Frame, error) {
	// If no response wanted, just send and return
	if expectFunc == nil {
		if err := sdoClient.SendRequest(req); err != nil {
			return nil, err
		}

		return nil, nil
	}

	// Set default timeout
	if timeout == nil {
		dtm := time.Duration(500) * time.Millisecond
		timeout = &dtm
	}

	if retryCount == nil {
		rtc := 4
		retryCount = &rtc
	}

	framesChan := sdoClient.Node.Network.AcquireFramesChan(expectFunc)

	// Retry loop
	remainingCount := *retryCount
	var frm *can.Frame

	for {
		if remainingCount == 0 {
			break
		}

		if err := sdoClient.SendRequest(req); err != nil {
			return nil, err
		}

		timer := time.NewTimer(*timeout)

		select {
		case <-timer.C:
			// Double timeout for each retry
			newTimeout := *timeout * 2
			timeout = &newTimeout
		case fr := <-framesChan.C:
			frm = fr
		}

		timer.Stop()
		remainingCount--

		if frm != nil {
			break
		}
	}

	// Release data chan
	sdoClient.Node.Network.ReleaseFramesChan(framesChan.ID)

	// If no frm, timeout execeded
	if frm == nil {
		return nil, errors.New("timeout execeded")
	}

	// The server refused the request, retrying will not change the answer
	if (frm.Data[0] & sdoCommandSpecifierMask) == SDOAbortTransfer {
		return nil, newSDOAbortError(frm)
	}

	return frm, nil
}

// Read sdo
func (sdoClient *SDOClient) Read(index uint16, subIndex uint8) ([]byte, error) {
	reader := NewSDOReader(sdoClient, index, subIndex)
	return reader.ReadAll()
}

// Write sdo
func (sdoClient *SDOClient) Write(index uint16, subIndex uint8, forceSegment bool, data []byte) error {
	writer := NewSDOWriter(sdoClient, index, subIndex, forceSegment)
	return writer.Write(data)
}
