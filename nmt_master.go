package canopen

import (
	"errors"
	"time"

	"github.com/angelodlfrtr/go-can"
	"github.com/google/uuid"
)

var NMTStates = map[int]string{
	0:   "INITIALISING",
	4:   "STOPPED",
	5:   "OPERATIONAL",
	80:  "SLEEP",
	96:  "STANDBY",
	127: "PRE-OPERATIONAL",
}

var NMTCommands = map[string]int{
	"OPERATIONAL":         1,
	"STOPPED":             2,
	"SLEEP":               80,
	"STANDBY":             96,
	"PRE-OPERATIONAL":     128,
	"INITIALISING":        129,
	"RESET":               129,
	"RESET COMMUNICATION": 130,
}

var NMTCommandToState = map[int]int{
	1:   5,
	2:   4,
	80:  80,
	96:  96,
	128: 127,
	129: 0,
	130: 0,
}

type NMTState struct {
	NodeID    int
	State     int
	Timestamp *time.Time
}

type NMTChangeChan struct {
	chanID string
	C      chan NMTState
}

type NMTMaster struct {
	NodeID        int
	Network       *Network
	State         int
	StateReceived *int
	Timestamp     *time.Time
	Listening     bool
	stopChan      chan bool

	ChangeChans []*NMTChangeChan

	// networkFramesChanID is used to store and later close the network frames channel
	networkFramesChanID *string
}

// NewNMTMaster return a new instance of Master
func NewNMTMaster(nodeID int, network *Network) *NMTMaster {
	return &NMTMaster{
		NodeID:      nodeID,
		Network:     network,
		stopChan:    make(chan bool, 1),
		ChangeChans: []*NMTChangeChan{},
	}
}

// UnlistenForHeartbeat listen message on network
func (master *NMTMaster) UnlistenForHeartbeat() error {
	if master.Network == nil {
		return errors.New("no network defined")
	}

	if master.networkFramesChanID == nil {
		return errors.New("not listening")
	}

	// Stop listen
	master.stopChan <- true

	// Release chan
	// The chan stop will have effect to close goroutine launched in ListenForHeartbeat
	master.Network.ReleaseFramesChan(*master.networkFramesChanID)

	master.Listening = false

	return nil
}

// ListenForHeartbeat listen message on network
func (master *NMTMaster) ListenForHeartbeat() error {
	if master.Network == nil {
		return errors.New("no network defined")
	}

	// Already listening ?
	if master.Listening {
		return nil
	}

	master.Listening = true

	// Hearbeat message arbID
	eventName := 0x700 + master.NodeID

	// Filter func for messages on network
	filterFunc := func(frm *can.Frame) bool {
		return frm.ArbitrationID == uint32(eventName)
	}

	// Get frames chan
	framesChan := master.Network.AcquireFramesChan(&filterFunc)
	master.networkFramesChanID = &framesChan.ID

	// Listen for messages
	go func() {
		for {
			select {
			case <-master.stopChan:
				// Stop goroutine
				return
			case frm := <-framesChan.C:
				master.handleHeartbeatFrame(frm)
			}
		}
	}()

	return nil
}

func (master *NMTMaster) handleHeartbeatFrame(frm *can.Frame) {
	now := time.Now()
	master.Timestamp = &now

	newState := int(frm.Data[0])
	changed := int(*&master.State) != int(newState)

	master.StateReceived = &newState

	if newState == 0 {
		master.State = 127
	} else {
		master.State = newState
	}

	if changed {
		for _, changeChan := range master.ChangeChans {
			select {
			case changeChan.C <- NMTState{NodeID: master.NodeID, State: master.State, Timestamp: master.Timestamp}:
			default:
			}
		}
	}
}

// SendCommand to target node
func (master *NMTMaster) SendCommand(code int) error {
	data := []byte{uint8(code), uint8(master.NodeID)}
	return master.Network.Send(0, data)
}

// SetState for target node, and send command
func (master *NMTMaster) SetState(cmd string) error {
	if _, ok := NMTCommands[cmd]; !ok {
		return errors.New("invalid NMT state")
	}

	code := NMTCommands[cmd]
	master.StateReceived = nil

	return master.SendCommand(code)
}

// GetStateString for target node
func (master *NMTMaster) GetStateString() string {
	if s, ok := NMTStates[master.State]; ok {
		return s
	}

	return ""
}

// WaitForBootup return when the node has *StateReceived == 0
// with a default timeout of 10s
func (master *NMTMaster) WaitForBootup(timeout *time.Duration) error {
	if timeout == nil {
		tmeout := time.Duration(10) * time.Second
		timeout = &tmeout
	}

	start := time.Now()

	for {
		if time.Since(start) > *timeout {
			return errors.New("timeout execeded")
		}

		if master.StateReceived != nil {
			if *master.StateReceived == 5 {
				break
			}
		}

		time.Sleep(time.Millisecond * 100)
	}

	return nil
}

// AcquireChangesChan create a new NMTChangeChan
func (master *NMTMaster) AcquireChangesChan() *NMTChangeChan {
	// Create frame chan
	chanID := uuid.Must(uuid.NewRandom()).String()
	changesChan := &NMTChangeChan{
		chanID: chanID,
		C:      make(chan NMTState),
	}

	// Append m.ChangeChans
	master.ChangeChans = append(master.ChangeChans, changesChan)

	return changesChan
}

// ReleaseChangesChan release (close) a NMTChangeChan
func (master *NMTMaster) ReleaseChangesChan(id string) error {
	var changesChan *NMTChangeChan
	var changesChanIndex *int

	for idx, fc := range master.ChangeChans {
		if fc.chanID == id {
			changesChan = fc
			idxx := idx
			changesChanIndex = &idxx
			break
		}
	}

	if changesChanIndex == nil {
		return errors.New("no NMTChangeChan found with specified ID")
	}

	// Close chan
	close(changesChan.C)

	// Remove frameChan from network.FramesChans
	master.ChangeChans = append(
		master.ChangeChans[:*changesChanIndex],
		master.ChangeChans[*changesChanIndex+1:]...,
	)

	return nil
}
