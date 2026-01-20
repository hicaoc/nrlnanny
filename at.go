package main

import "strings"

type atCommand struct {
	command string
	value   string
}

func decodeAT(data []byte) *atCommand {
	if len(data) < 2 {
		return nil
	}

	if data[0] != 0x01 {
		return nil
	}

	str := strings.Split(string(data[1:]), "=")

	if len(str) != 2 {
		return nil
	}

	at := atCommand{}

	at.command = str[0]

	at.value = strings.TrimSuffix(str[1], "\r\n")

	return &at

}

func encodeAT(atlist []string) []byte {
	at := make([]byte, 0)
	at = append(at, 0x02)
	at = append(at, "NRLNANNY V2.0\r\n"...)
	at = append(at, strings.Join(atlist, "\r\n")...)
	at = append(at, "\r\n"...)

	return at

}
