//
// Copyright (c) Kevin Johnson 2018
//

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/jamescun/tuntap"
)

//
// This program opens the tap0 network interface and transfers packets to and from a local socket
// The local socket should be forwarded using ssh to the remote server
// The remote server connects the home and work, making the local tap0 interface appear on the work lan
// This requires:
//   modprobe tun
//   ssh -L 127.0.0.1:2000:127.0.0.1:2000 root@www.renoswe.com
// On the remote side
//   modprobe tun
//   brctl addbr br0
//   tunctl -t tap0
//   ifconfig tap0 up
//   ifconfig enp30s0:1 up
//   brctl addif br0 tap0
//   brctl addif br0 enp30s0:1
//   ssh -N -p 2222 -R 127.0.0.1:2000:127.0.0.1:2000 root@www.renoswe.com
//
//

func main() {
	tun, err := tuntap.Tap("tap0")
	if err != nil {
		fmt.Println("error: tap:", err)
		return
	}

	defer tun.Close()

	if len(os.Args) == 2 && os.Args[1] == "home" {
		connectToRemote(tun)
	} else {
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

		_, err = conn.Write(buf[:n+4])
		if err != nil {
			fmt.Println("error: write net:", err)
			break
		}
	}

	done <- true
}

func recvFromRemote(tun tuntap.Interface, conn net.Conn, done chan bool) {
	buf := make([]byte, 1600)
	for {
		var len uint32

		n, err := conn.Read(buf[0:4])
		if err != nil {
			fmt.Println("error: read net:", err)
			break
		}

		len = binary.BigEndian.Uint32(buf[0:4])
		n, err = conn.Read(buf[4 : len+4])
		if err != nil {
			fmt.Println("error: read net:", err)
			break
		}

		_, err = tun.Write(buf[4 : n+4])
		if err != nil {
			fmt.Println("error: write tap:", err)
			break
		}
	}

	done <- true
}
