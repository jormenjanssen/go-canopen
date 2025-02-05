package canopen

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/angelodlfrtr/go-can"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	bigData       = "SizeLongerAsOneLine"
	smallData     = "Line"
	sizeBigData   = uint32(19)
	sizeSmallData = uint32(4)
)

func getSDOClientMockExpeditedSuccess() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(&frame1, nil)
	return client
}

func getSDOClientMockExpeditedFailed() *SDOClientMock {
	client := &SDOClientMock{}
	client.On("Send", []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65}).Return(nil, errors.New("Failed to send frame"))
	return client
}

func getSDOClientMockSegmentedSuccess() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	frame2 := can.Frame{Data: [8]byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	frame3 := can.Frame{Data: [8]byte{0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	frame4 := can.Frame{Data: [8]byte{0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	client.On("Send", []byte{0x00, 0x53, 0x69, 0x7A, 0x65, 0x4C, 0x6F, 0x6E}).Return(&frame2, nil)
	client.On("Send", []byte{0x10, 0x67, 0x65, 0x72, 0x41, 0x73, 0x4F, 0x6E}).Return(&frame3, nil)
	client.On("Send", []byte{0x05, 0x65, 0x4C, 0x69, 0x6E, 0x65, 0x00, 0x00}).Return(&frame4, nil)
	return client
}

func getSDOClientMockSegmentedFailed1() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x00, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	return client
}

func getSDOClientMockSegmentedFailed2() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	return client
}

func getSDOClientMockSegmentedFailed3() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	return client
}

func getSDOClientMockSegmentedFailed4() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	frame2 := can.Frame{Data: [8]byte{0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	client.On("Send", []byte{0x00, 0x53, 0x69, 0x7A, 0x65, 0x4C, 0x6F, 0x6E}).Return(&frame2, nil)
	return client
}

func getSDOClientMockSegmentedFailed5() *SDOClientMock {
	client := &SDOClientMock{}
	frame1 := can.Frame{Data: [8]byte{0x60, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00}}
	frame2 := can.Frame{Data: [8]byte{0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}
	client.On("Send", []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00}).Return(&frame1, nil)
	client.On("Send", []byte{0x00, 0x53, 0x69, 0x7A, 0x65, 0x4C, 0x6F, 0x6E}).Return(&frame2, nil)
	return client
}

type SDOClientMock struct {
	mock.Mock
}

func (s *SDOClientMock) FindName(name string) DicObject {
	args := s.Called(name)
	return args.Get(0).(DicObject)
}

func (s *SDOClientMock) Read(index uint16, subIndex uint8) ([]byte, error) {
	args := s.Called(index, subIndex)
	return args.Get(0).([]byte), args.Error(1)
}

func (s *SDOClientMock) Send(req []byte, expectFunc networkFramesChanFilterFunc, timeout *time.Duration, retryCount *int) (*can.Frame, error) {
	args := s.Called(req)
	argFrame := args.Get(0)
	if argFrame == nil {
		return nil, args.Error(1)
	}
	returnFrame := argFrame.(*can.Frame)
	if expectFunc != nil {
		checkFunc := *expectFunc
		if !checkFunc(returnFrame) {
			fmt.Println(req)
			fmt.Println(returnFrame.Data)
			return nil, errors.New("Frame not matched")
		}
	}
	return returnFrame, args.Error(1)
}

func (s *SDOClientMock) SendRequest(req []byte) error {
	args := s.Called(req)
	return args.Error(0)
}

func (s *SDOClientMock) Write(index uint16, subIndex uint8, forceSegment bool, data []byte) error {
	args := s.Called(index, subIndex, forceSegment, data)
	return args.Error(0)
}

func TestSDOWriter_buildRequestDownloadBuf(t *testing.T) {
	type fields struct {
		SDOClient    ISDOClient
		Index        uint16
		SubIndex     uint8
		ForceSegment bool
	}
	type args struct {
		data []byte
		size *uint32
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantType  string
		wantBytes []byte
	}{
		{
			name: "Size not specified",
			fields: fields{
				SDOClient:    nil,
				Index:        0x3E8, //1000
				SubIndex:     0x02,
				ForceSegment: false,
			},
			args: args{
				data: []byte(bigData),
				size: nil,
			},
			wantType:  "segmented",
			wantBytes: []byte{0x20, 0xE8, 0x03, 0x02, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "Size specified",
			fields: fields{
				SDOClient:    nil,
				Index:        0x3E8, //1000
				SubIndex:     0x02,
				ForceSegment: false,
			},
			args: args{
				data: []byte(bigData),
				size: &sizeBigData,
			},
			wantType:  "segmented",
			wantBytes: []byte{0x21, 0xE8, 0x03, 0x02, 0x13, 0x00, 0x00, 0x00},
		},
		{
			name: "ForceSegmented",
			fields: fields{
				SDOClient:    nil,
				Index:        0x3E8, //1000
				SubIndex:     0x02,
				ForceSegment: true,
			},
			args: args{
				data: []byte(smallData),
				size: &sizeSmallData,
			},
			wantType:  "segmented",
			wantBytes: []byte{0x21, 0xE8, 0x03, 0x02, 0x04, 0x00, 0x00, 0x00},
		},
		{
			name: "Expedited",
			fields: fields{
				SDOClient:    nil,
				Index:        0x3E8, //1000
				SubIndex:     0x02,
				ForceSegment: false,
			},
			args: args{
				data: []byte(smallData),
				size: &sizeSmallData,
			},
			wantType:  "expedited",
			wantBytes: []byte{0x23, 0xE8, 0x03, 0x02, 0x4C, 0x69, 0x6E, 0x65},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewSDOWriter(tt.fields.SDOClient, tt.fields.Index, tt.fields.SubIndex, tt.fields.ForceSegment)
			gotType, gotBytes := writer.buildRequestDownloadBuf(tt.args.data, tt.args.size)
			if gotType != tt.wantType {
				t.Errorf("SDOWriter.buildRequestDownloadBuf() gotType = %v, wantType %v", gotType, tt.wantType)
			}
			if !assert.Equal(t, tt.wantBytes, gotBytes) {
				t.Errorf("SDOWriter.buildRequestDownloadBuf() gotBytes = %v, wantBytes %v", gotBytes, tt.wantBytes)
			}
		})
	}
}

func TestSDOWriter_RequestDownload(t *testing.T) {
	type fields struct {
		SDOClientFunc func() *SDOClientMock
		Index         uint16
		SubIndex      uint8
		ForceSegment  bool
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Write expedited success",
			fields: fields{
				SDOClientFunc: getSDOClientMockExpeditedSuccess,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(smallData),
			},
			wantErr: false,
		},
		{
			name: "Write expedited failed",
			fields: fields{
				SDOClientFunc: getSDOClientMockExpeditedFailed,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(smallData),
			},
			wantErr: true,
		},
		{
			name: "Write segmented success",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedSuccess,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: false,
		},
		{
			name: "Write segmented failed response",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedFailed1,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: true,
		},
		{
			name: "Write segmented failed index",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedFailed2,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: true,
		},
		{
			name: "Write segmented failed subindex",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedFailed3,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: true,
		},
		{
			name: "Write segmented failed segmented response",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedFailed4,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: true,
		},
		{
			name: "Write segmented failed toggle bit",
			fields: fields{
				SDOClientFunc: getSDOClientMockSegmentedFailed5,
				Index:         0x3E8, //1000
				SubIndex:      0x02,
				ForceSegment:  false,
			},
			args: args{
				data: []byte(bigData),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := NewSDOWriter(tt.fields.SDOClientFunc(), tt.fields.Index, tt.fields.SubIndex, tt.fields.ForceSegment)
			if err := writer.RequestDownload(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("SDOWriter.RequestDownload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
