package mysqldriver

import (
	"fmt"
	"net"

	"github.com/pubnative/mysqlproto-go"
)

type Conn struct {
	stream *mysqlproto.Stream
}

type Stats struct {
	Syscalls int
}

func NewConn(username, password, protocol, address, database string) (Conn, error) {
	conn, err := net.Dial(protocol, address)
	if err != nil {
		return Conn{}, err
	}

	stream := mysqlproto.NewStream(conn)

	if err = handshake(stream, username, password, database); err != nil {
		return Conn{}, err
	}

	if err = setUTF8Charset(stream); err != nil {
		return Conn{}, err
	}

	return Conn{stream}, nil
}

func (c Conn) Close() error {
	return c.stream.Close()
}

func (c Conn) Stats() Stats {
	return Stats{
		Syscalls: c.stream.Syscalls(),
	}
}

func (s Stats) Add(stats Stats) Stats {
	return Stats{
		Syscalls: s.Syscalls + stats.Syscalls,
	}
}

func handshake(stream *mysqlproto.Stream, username, password, database string) error {
	packet, err := mysqlproto.ReadHandshakeV10(stream)
	if err != nil {
		return err
	}

	res := mysqlproto.HandshakeResponse41(
		packet.CapabilityFlags,
		packet.CharacterSet,
		username,
		password,
		packet.AuthPluginData,
		database,
		packet.AuthPluginName,
		nil,
	)

	if _, err := stream.Write(res); err != nil {
		return err
	}

	pkt, err := stream.NextPacket()
	if err != nil {
		return err
	}

	return handleOK(pkt.Payload)
}

func setUTF8Charset(stream *mysqlproto.Stream) error {
	data := mysqlproto.ComQueryRequest([]byte("SET NAMES utf8"))
	if _, err := stream.Write(data); err != nil {
		return err
	}

	packet, err := stream.NextPacket()
	if err != nil {
		return err
	}

	return handleOK(packet.Payload)
}

func handleOK(payload []byte) error {
	if payload[0] == mysqlproto.PACKET_OK {
		return nil
	}

	if payload[0] == mysqlproto.PACKET_ERR {
		errPacket, err := mysqlproto.ParseERRPacket(payload)
		if err != nil {
			return err
		}
		return errPacket
	}

	return fmt.Errorf("mysqldriver: unknown error occured. Payload: %x", payload)
}
