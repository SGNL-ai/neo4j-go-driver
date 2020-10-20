/*
 * Copyright (c) 2002-2020 "Neo4j,"
 * Neo4j Sweden AB [http://neo4j.com]
 *
 * This file is part of Neo4j.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package bolt

import (
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j/internal/packstream"
)

// Fake of bolt4 server.
// Utility to test bolt4 protocol implemntation.
// Use panic upon errors, simplifies output when server is running within a go thread
// in the test.
type bolt4server struct {
	conn     net.Conn
	hyd      packstream.Hydrate
	unpacker *packstream.Unpacker
	chunker  *chunker
	packer   *packstream.Packer
}

func newBolt4Server(conn net.Conn) *bolt4server {
	return &bolt4server{
		unpacker: &packstream.Unpacker{},
		conn:     conn,
		chunker:  newChunker(),
		hyd:      passthroughHydrator,
		packer:   &packstream.Packer{},
	}
}

func (s *bolt4server) waitForHandshake() []byte {
	handshake := make([]byte, 4*5)
	_, err := io.ReadFull(s.conn, handshake)
	if err != nil {
		panic(err)
	}
	return handshake
}

func (s *bolt4server) assertStructType(msg *packstream.Struct, t packstream.StructTag) {
	if msg.Tag != t {
		panic(fmt.Sprintf("Got wrong type of message expected %d but got %d (%+v)", t, msg.Tag, msg))
	}
}

func (s *bolt4server) sendFailureMsg(code, msg string) {
	f := map[string]interface{}{
		"code":    code,
		"message": msg,
	}
	s.send(msgFailure, f)
}

func (s *bolt4server) sendIgnoredMsg() {
	s.send(msgIgnored)
}

// Returns the first hello field
func (s *bolt4server) waitForHello() map[string]interface{} {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgHello)
	m := msg.Fields[0].(map[string]interface{})
	// Hello should contain some musts
	_, exists := m["scheme"]
	if !exists {
		s.sendFailureMsg("?", "Missing scheme in hello")
	}
	_, exists = m["user_agent"]
	if !exists {
		s.sendFailureMsg("?", "Missing user_agent in hello")
	}
	return m
}

func (s *bolt4server) receiveMsg() *packstream.Struct {
	buf, err := dechunkMessage(s.conn, []byte{})
	if err != nil {
		panic(err)
	}
	x, err := s.unpacker.UnpackStruct(buf, s.hyd)
	if err != nil {
		panic(err)
	}
	return x.(*packstream.Struct)
}

func (s *bolt4server) waitForRun() {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgRun)
}

func (s *bolt4server) waitForReset() {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgReset)
}

func (s *bolt4server) waitForTxBegin() {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgBegin)
}

func (s *bolt4server) waitForTxCommit() {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgCommit)
}

func (s *bolt4server) waitForTxRollback() {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgRollback)
}

func (s *bolt4server) waitForPullN(n int) {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgPullN)
	extra := msg.Fields[0].(map[string]interface{})
	sentN := int(extra["n"].(int64))
	if sentN != n {
		panic(fmt.Sprintf("Expected PULL n:%d but got PULL %d", n, sentN))
	}
	_, hasQid := extra["qid"]
	if hasQid {
		panic("Expected PULL without qid")
	}
}

func (s *bolt4server) waitForPullNandQid(n, qid int) {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgPullN)
	extra := msg.Fields[0].(map[string]interface{})
	sentN := int(extra["n"].(int64))
	if sentN != n {
		panic(fmt.Sprintf("Expected PULL n:%d but got PULL %d", n, sentN))
	}
	sentQid := int(extra["qid"].(int64))
	if sentQid != qid {
		panic(fmt.Sprintf("Expected PULL qid:%d but got PULL %d", qid, sentQid))
	}
}
func (s *bolt4server) waitForDiscardN(n, qid int) {
	msg := s.receiveMsg()
	s.assertStructType(msg, msgDiscardN)
	extra := msg.Fields[0].(map[string]interface{})
	sentN := int(extra["n"].(int64))
	if sentN != n {
		panic(fmt.Sprintf("Expected DISCARD n:%d but got DISCARD %d", n, sentN))
	}
	sentQid := int(extra["qid"].(int64))
	if sentQid != qid {
		panic(fmt.Sprintf("Expected DISCARD qid:%d but got DISCARD %d", qid, sentQid))
	}
}

func (s *bolt4server) acceptVersion(ver byte) {
	acceptedVer := []byte{0x00, 0x00, 0x00, ver}
	_, err := s.conn.Write(acceptedVer)
	if err != nil {
		panic(err)
	}
}

func (s *bolt4server) rejectVersions() {
	_, err := s.conn.Write([]byte{0x00, 0x00, 0x00, 0x00})
	if err != nil {
		panic(err)
	}
}

func (s *bolt4server) closeConnection() {
	s.conn.Close()
}

func (s *bolt4server) send(tag packstream.StructTag, field ...interface{}) {
	s.chunker.beginMessage()
	var err error
	s.chunker.buf, err = s.packer.PackStruct(s.chunker.buf, dehydrate, tag, field...)
	if err != nil {
		panic(err)
	}
	s.chunker.endMessage()
	s.chunker.send(s.conn)
}

func (s *bolt4server) sendSuccess(m map[string]interface{}) {
	s.send(msgSuccess, m)
}

func (s *bolt4server) acceptHello() {
	s.send(msgSuccess, map[string]interface{}{
		"connection_id": "cid",
		"server":        "fake/4.5",
	})
}

func (s *bolt4server) rejectHelloUnauthorized() {
	s.send(msgFailure, map[string]interface{}{
		"code":    "Neo.ClientError.Security.Unauthorized",
		"message": "",
	})
}

// Utility when something else but connect is to be tested
func (s *bolt4server) accept(ver byte) {
	s.waitForHandshake()
	s.acceptVersion(ver)
	s.waitForHello()
	s.acceptHello()
}

// Utility to wait and serve a auto commit query
func (s *bolt4server) serveRun(stream []packstream.Struct) {
	s.waitForRun()
	s.waitForPullN(bolt4_fetchsize)
	for _, x := range stream {
		s.send(x.Tag, x.Fields...)
	}
}

func (s *bolt4server) serveRunTx(stream []packstream.Struct, commit bool, bookmark string) {
	s.waitForTxBegin()
	s.send(msgSuccess, map[string]interface{}{})
	s.waitForRun()
	s.waitForPullN(bolt4_fetchsize)
	for _, x := range stream {
		s.send(x.Tag, x.Fields...)
	}
	if commit {
		s.waitForTxCommit()
		s.send(msgSuccess, map[string]interface{}{
			"bookmark": bookmark,
		})
	} else {
		s.waitForTxRollback()
		s.send(msgSuccess, map[string]interface{}{})
	}
}

func setupBolt4Pipe(t *testing.T) (net.Conn, *bolt4server, func()) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Unable to listen: %s", err)
	}

	addr := l.Addr()
	clientConn, err := net.Dial(addr.Network(), addr.String())

	srvConn, err := l.Accept()
	if err != nil {
		t.Fatalf("Accept error: %s", err)
	}
	srv := newBolt4Server(srvConn)

	return clientConn, srv, func() {
		l.Close()
	}
}
