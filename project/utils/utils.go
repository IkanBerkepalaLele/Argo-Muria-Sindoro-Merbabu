package utils

import (
	"encoding/binary"
)

// Component data types may be modified, but field names must not be changed
type LRTJPIDSPacketFixed struct {
	TransactionId          uint16
	IsAck                  uint8
	IsNewTrain             uint8
	IsUpdateTrain          uint8
	IsDeleteTrain          uint8
	IsTrainArriving        uint8
	IsTrainDeparting       uint8
	TrainNumber            uint16
	EstimatedArrivalHour   uint8
	EstimatedArrivalMinute uint8
	DestinationLength      uint8
}

type LRTJPIDSPacket struct {
	LRTJPIDSPacketFixed
	Destination string
}

func Encoder(packet LRTJPIDSPacket) []byte {
	destination := []byte(packet.Destination)
	destinationLength := len(destination)

	rawMessage := make([]byte, 8+destinationLength)

	binary.BigEndian.PutUint16(rawMessage[0:2], packet.TransactionId)

	var flags uint8
	flags |= (packet.IsAck & 1) << 7
	flags |= (packet.IsNewTrain & 1) << 6
	flags |= (packet.IsUpdateTrain & 1) << 5
	flags |= (packet.IsDeleteTrain & 1) << 4
	flags |= (packet.IsTrainArriving & 1) << 3
	flags |= (packet.IsTrainDeparting & 1) << 2

	rawMessage[2] = flags

	binary.BigEndian.PutUint16(rawMessage[3:5], packet.TrainNumber)

	rawMessage[5] = packet.EstimatedArrivalHour
	rawMessage[6] = packet.EstimatedArrivalMinute
	rawMessage[7] = uint8(destinationLength)

	copy(rawMessage[8:], destination)

	return rawMessage
}

func Decoder(rawMessage []byte) LRTJPIDSPacket {
	if len(rawMessage) < 8 {
		return LRTJPIDSPacket{}
	}

	flags := rawMessage[2]
	destinationLength := int(rawMessage[7])

	if len(rawMessage) < 8+destinationLength {
		return LRTJPIDSPacket{}
	}

	packet := LRTJPIDSPacket{
		LRTJPIDSPacketFixed: LRTJPIDSPacketFixed{
			TransactionId:          binary.BigEndian.Uint16(rawMessage[0:2]),
			IsAck:                  (flags >> 7) & 1,
			IsNewTrain:             (flags >> 6) & 1,
			IsUpdateTrain:          (flags >> 5) & 1,
			IsDeleteTrain:          (flags >> 4) & 1,
			IsTrainArriving:        (flags >> 3) & 1,
			IsTrainDeparting:       (flags >> 2) & 1,
			TrainNumber:            binary.BigEndian.Uint16(rawMessage[3:5]),
			EstimatedArrivalHour:   rawMessage[5],
			EstimatedArrivalMinute: rawMessage[6],
			DestinationLength:      rawMessage[7],
		},
		Destination: string(rawMessage[8 : 8+destinationLength]),
	}

	return packet
}
