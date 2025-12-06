package core

import (
	"github.com/vuuvv/errors"
	"github.com/vuuvv/vpacket/crc16"
	"strings"
)

const (
	KEY_ARC         = "arc"
	KEY_AUG_CCITT   = "aug_ccitt"
	KEY_BUYPASS     = "buypass"
	KEY_CCITT_FALSE = "ccitt_false"
	KEY_CDMA2000    = "cdma2000"
	KEY_DDS_110     = "dds_110"
	KEY_DECT_R      = "dect_r"
	KEY_DECT_X      = "dect_x"
	KEY_DNP         = "dnp"
	KEY_EN_13757    = "en_13757"
	KEY_GENIBUS     = "genibus"
	KEY_MAXIM       = "maxim"
	KEY_MCRF4XX     = "mcrf4xx"
	KEY_RIELLO      = "riello"
	KEY_T10_DIF     = "t10_dif"
	KEY_TELEDISK    = "teledisk"
	KEY_TMS37157    = "tms37157"
	KEY_USB         = "usb"
	KEY_CRC_A       = "crc_a"
	KEY_KERMIT      = "kermit"
	KEY_MODBUS      = "modbus"
	KEY_X_25        = "x_25"
	KEY_XMODEM      = "xmodem"
)

var Crc16Map = map[string]int{
	KEY_ARC:         0,
	KEY_AUG_CCITT:   1,
	KEY_BUYPASS:     2,
	KEY_CCITT_FALSE: 3,
	KEY_CDMA2000:    4,
	KEY_DDS_110:     5,
	KEY_DECT_R:      6,
	KEY_DECT_X:      7,
	KEY_DNP:         8,
	KEY_EN_13757:    9,
	KEY_GENIBUS:     10,
	KEY_MAXIM:       11,
	KEY_MCRF4XX:     12,
	KEY_RIELLO:      13,
	KEY_T10_DIF:     14,
	KEY_TELEDISK:    15,
	KEY_TMS37157:    16,
	KEY_USB:         17,
	KEY_CRC_A:       18,
	KEY_KERMIT:      19,
	KEY_MODBUS:      20,
	KEY_X_25:        21,
	KEY_XMODEM:      22,
}

// Crc 计算crc, name的格式 xxx_xxx, 如crc16_modbus
func Crc(data []byte, name string) (uint64, error) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) < 2 {
		return 0, errors.Errorf("invalid crc name: %s", name)
	}
	crcType := parts[0]
	crcName := parts[1]
	switch crcType {
	case "crc16":
		return Crc16(data, crcName)
	default:
		return 0, errors.Errorf("unsupport crc bits: %s", crcType)
	}
}

func Crc16(data []byte, key string) (uint64, error) {
	if len(data) == 0 {
		return 0, nil
	}
	n, ok := Crc16Map[key]
	if !ok {
		return 0, errors.Errorf("crc16: unsupport crc type '%s'", key)
	}
	return uint64(crc16.Checksum(data, n)), nil
}
