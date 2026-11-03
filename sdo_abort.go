package canopen

import (
	"encoding/binary"
	"fmt"

	"github.com/angelodlfrtr/go-can"
)

// SDO abort codes as defined by CiA 301
const (
	SDOAbortToggleBitNotAlternated   uint32 = 0x05030000
	SDOAbortTimeout                  uint32 = 0x05040000
	SDOAbortInvalidCommandSpecifier  uint32 = 0x05040001
	SDOAbortOutOfMemory              uint32 = 0x05040005
	SDOAbortUnsupportedAccess        uint32 = 0x06010000
	SDOAbortReadWriteOnly            uint32 = 0x06010001
	SDOAbortWriteReadOnly            uint32 = 0x06010002
	SDOAbortObjectDoesNotExist       uint32 = 0x06020000
	SDOAbortObjectNotMappable        uint32 = 0x06040041
	SDOAbortPDOLengthExceeded        uint32 = 0x06040042
	SDOAbortParameterIncompatible    uint32 = 0x06040043
	SDOAbortDeviceIncompatible       uint32 = 0x06040047
	SDOAbortHardwareError            uint32 = 0x06060000
	SDOAbortLengthMismatch           uint32 = 0x06070010
	SDOAbortLengthTooHigh            uint32 = 0x06070012
	SDOAbortLengthTooLow             uint32 = 0x06070013
	SDOAbortSubIndexDoesNotExist     uint32 = 0x06090011
	SDOAbortInvalidValue             uint32 = 0x06090030
	SDOAbortValueTooHigh             uint32 = 0x06090031
	SDOAbortValueTooLow              uint32 = 0x06090032
	SDOAbortMaxLessThanMin           uint32 = 0x06090036
	SDOAbortResourceNotAvailable     uint32 = 0x060A0023
	SDOAbortGeneralError             uint32 = 0x08000000
	SDOAbortDataNotTransferable      uint32 = 0x08000020
	SDOAbortDataNotTransferableLocal uint32 = 0x08000021
	SDOAbortDataNotTransferableState uint32 = 0x08000022
	SDOAbortNoObjectDictionary       uint32 = 0x08000023
	SDOAbortNoDataAvailable          uint32 = 0x08000024
)

var sdoAbortDescriptions = map[uint32]string{
	SDOAbortToggleBitNotAlternated:   "toggle bit not alternated",
	SDOAbortTimeout:                  "SDO protocol timed out",
	SDOAbortInvalidCommandSpecifier:  "client/server command specifier not valid or unknown",
	SDOAbortOutOfMemory:              "out of memory",
	SDOAbortUnsupportedAccess:        "unsupported access to an object",
	SDOAbortReadWriteOnly:            "attempt to read a write only object",
	SDOAbortWriteReadOnly:            "attempt to write a read only object",
	SDOAbortObjectDoesNotExist:       "object does not exist in the object dictionary",
	SDOAbortObjectNotMappable:        "object cannot be mapped to the PDO",
	SDOAbortPDOLengthExceeded:        "number and length of the objects to be mapped would exceed PDO length",
	SDOAbortParameterIncompatible:    "general parameter incompatibility",
	SDOAbortDeviceIncompatible:       "general internal incompatibility in the device",
	SDOAbortHardwareError:            "access failed due to a hardware error",
	SDOAbortLengthMismatch:           "data type does not match, length of service parameter does not match",
	SDOAbortLengthTooHigh:            "data type does not match, length of service parameter too high",
	SDOAbortLengthTooLow:             "data type does not match, length of service parameter too low",
	SDOAbortSubIndexDoesNotExist:     "subindex does not exist",
	SDOAbortInvalidValue:             "invalid value for parameter",
	SDOAbortValueTooHigh:             "value of parameter written too high",
	SDOAbortValueTooLow:              "value of parameter written too low",
	SDOAbortMaxLessThanMin:           "maximum value is less than minimum value",
	SDOAbortResourceNotAvailable:     "resource not available: SDO connection",
	SDOAbortGeneralError:             "general error",
	SDOAbortDataNotTransferable:      "data cannot be transferred or stored to the application",
	SDOAbortDataNotTransferableLocal: "data cannot be transferred or stored to the application because of local control",
	SDOAbortDataNotTransferableState: "data cannot be transferred or stored because of the present device state",
	SDOAbortNoObjectDictionary:       "object dictionary dynamic generation fails or no object dictionary is present",
	SDOAbortNoDataAvailable:          "no data available",
}

// SDOAbortError is returned when the SDO server answers a request with an abort transfer
type SDOAbortError struct {
	Index    uint16
	SubIndex uint8
	Code     uint32
}

// Description returns the CiA 301 description of the abort code, or an empty string for an unknown code
func (abortErr *SDOAbortError) Description() string {
	return sdoAbortDescriptions[abortErr.Code]
}

func (abortErr *SDOAbortError) Error() string {
	msg := fmt.Sprintf(
		"sdo abort 0x%08X on index 0x%04X subindex 0x%02X",
		abortErr.Code,
		abortErr.Index,
		abortErr.SubIndex,
	)

	if description := abortErr.Description(); description != "" {
		msg += ": " + description
	}

	return msg
}

// newSDOAbortError decodes the object and abort code of an abort transfer frame
func newSDOAbortError(frm *can.Frame) *SDOAbortError {
	return &SDOAbortError{
		Index:    binary.LittleEndian.Uint16(frm.Data[1:]),
		SubIndex: frm.Data[3],
		Code:     binary.LittleEndian.Uint32(frm.Data[4:]),
	}
}

// sdoFrameMatchesObject reports whether the multiplexer of frm addresses index and subIndex
func sdoFrameMatchesObject(frm *can.Frame, index uint16, subIndex uint8) bool {
	return binary.LittleEndian.Uint16(frm.Data[1:]) == index && frm.Data[3] == subIndex
}
