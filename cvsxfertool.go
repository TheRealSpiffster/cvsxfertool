//
// Copyright (c) Kevin Johnson 2018
//

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/jamescun/tuntap"
)

//
// This program opens the tap0 network interface and transfers packets to and from a local socket
// The local socket should be forwarded using ssh to the remote server
// The remote server connects the home and work, making the local tap0 interface appear on the work lan
// This requires:
//   modprobe tun
//   ssh -L 127.0.0.1:2000:127.0.0.1:2000 root@HOME
//   cvsxfertool home &
//   # ifconfig tap0 up   (now done internally)
//   dhclient -v -4 tap0
//
// On the remote side
//   # requrement: set up bridge bridge0 with eth0 interface and make sure it gets a dhcp
//   modprobe tun
//   ssh -N -R 127.0.0.1:2000:127.0.0.1:2000 root@HOME
//   cvsxfertool &
//   # ifconfig tap0 up (now done internally)
//   # brctl addif bridge0 tap0 (now done internally)
//
//

func main() {
	tun, err := tuntap.Tap("tap0")
	if err != nil {
		fmt.Println("error: tap:", err)
		return
	}

	cmd := exec.Command("/sbin/ifconfig", "tap0", "up")
	cmd.Run()

	defer tun.Close()

	if len(os.Args) == 2 && os.Args[1] == "home" {
		connectToRemote(tun)
	} else {
		cmd = exec.Command("/sbin/brctl", "addif", "bridge0", "tap0")
		cmd.Run()
		serverFromRemote(tun)
	}
}

func connectToRemote(tun tuntap.Interface) {
	for {
		time.Sleep(5 * time.Second)
		fmt.Println("Connecting...")
		conn, err := net.Dial("tcp", "127.0.0.1:2000")
		if err != nil {
			fmt.Println("error: connect tcp: ", err)
		} else {
			doChan(tun, conn)
		}
	}
}

func serverFromRemote(tun tuntap.Interface) {
	for {
		fmt.Println("waiting for connection...")
		s, err := net.Listen("tcp", "127.0.0.1:2000")
		if err != nil {
			fmt.Println("Error listening:", err)
			return
		}

		defer s.Close()
		for {
			conn, err := s.Accept()
			if err != nil {
				fmt.Println("Error accepting connection:", err)
			} else {
				go doChan(tun, conn)
			}
		}
	}
}

func doChan(tun tuntap.Interface, conn net.Conn) {
	done := make(chan bool, 1)

	go sendToRemote(tun, conn, done)
	go recvFromRemote(tun, conn, done)

	// read two messages, one from each goroutine
	<-done
	conn.Close()
	<-done
}

func sendToRemote(tun tuntap.Interface, conn net.Conn, done chan bool) {
	buf := make([]byte, 1600)
	for {
		n, err := tun.Read(buf[4:])
		if err == tuntap.ErrNotReady {
			fmt.Println("warning: tap: interface not ready, waiting 1s...")
			time.Sleep(1 * time.Second)
			continue
		} else if err != nil {
			fmt.Println("error: read tap:", err)
			break
		}

		binary.BigEndian.PutUint32(buf, uint32(n))

		if !putDataToConn(conn, buf, 0, n+4) {
			break
		}
	}

	done <- true
}

func recvFromRemote(tun tuntap.Interface, conn net.Conn, done chan bool) {
	buf := make([]byte, 1600)
	for {

		if !getDataFromConn(conn, buf, 0, 4) {
			break
		}

		blen := int(binary.BigEndian.Uint32(buf[0:4]))

		if blen+4 > len(buf) {
			fmt.Println("Error: received packet size", blen, " more than", len(buf)-4)
		}

		if !getDataFromConn(conn, buf, 4, blen) {
			break
		}

		_, err := tun.Write(buf[4 : blen+4])
		if err != nil {
			fmt.Println("error: write tap:", err)
			break
		}
	}

	done <- true
}

func getDataFromConn(conn net.Conn, buf []byte, loc, size int) bool {
	bytesReceived := 0
	for bytesReceived < size {
		n, err := conn.Read(buf[loc+bytesReceived : loc+size])
		if err != nil {
			fmt.Println("error: read net:", err)
			return false
		}

		bytesReceived += n
	}

	return true
}

func putDataToConn(conn net.Conn, buf []byte, loc, size int) bool {
	bytesSent := 0
	for bytesSent < size {
		n, err := conn.Write(buf[loc+bytesSent : loc+size])

		if err != nil {
			fmt.Println("error: read net:", err)
			return false
		}

		bytesSent += n
	}

	return true
}
