/*
 * Copyright (c) "Neo4j"
 * Neo4j Sweden AB [https://neo4j.com]
 *
 * This file is part of Neo4j.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
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
	"reflect"
	"testing"
	"time"

	idb "github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/internal/db"

	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/db"
	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/dbtype"
	"github.com/SGNL-ai/neo4j-go-driver/v5/neo4j/internal/packstream"
)

type hydratorTestCase struct {
	name   string
	build  func() // Builds/encodes stream same was as server would
	x      any    // Expected hydrated
	err    error
	useUtc bool
}

func TestHydrator(outer *testing.T) {
	zoneName := "America/New_York"
	timeZone, err := time.LoadLocation(zoneName)
	if err != nil {
		panic(err)
	}

	packer := packstream.Packer{}
	cases := []hydratorTestCase{
		{
			name: "Ignored",
			build: func() {
				packer.StructHeader(byte(msgIgnored), 0)
			},
			x: &ignored{},
		},
		{
			name: "Error",
			build: func() {
				packer.StructHeader(byte(msgFailure), 0)
				packer.MapHeader(3)
				packer.String("code")
				packer.String("the code")
				packer.String("message")
				packer.String("mess")
				packer.String("extra key") // Should be ignored
				packer.Int(1)
			},
			err: &db.ProtocolError{MessageType: "failure", Err: "Invalid length of struct, expected 1 but was 0"},
		},
		{
			name: "Success hello response",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(3)
				packer.String("connection_id")
				packer.String("connid")
				packer.String("server")
				packer.String("srv")
				packer.String("details") // Should be ignored
				packer.Int8(1)
			},
			x: &success{tlast: -1, tfirst: -1, connectionId: "connid", server: "srv", qid: -1, num: 3},
		},
		{
			name: "Success commit/rollback/reset response",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(0)
			},
			x: &success{tlast: -1, tfirst: -1, qid: -1, num: 0},
		},
		{
			name: "Success run response",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(3)
				packer.String("unknown") // Should be ignored
				packer.Int64(666)
				packer.String("fields")
				packer.ArrayHeader(2)   // >> fields array
				packer.String("field1") //
				packer.String("field2") // << fields array
				packer.String("t_first")
				packer.Int64(10000)
			},
			x: &success{tlast: -1, fields: []string{"field1", "field2"}, tfirst: 10000, qid: -1, num: 3},
		},
		{
			name: "Success run response with qid",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("unknown") // Should be ignored
				packer.Int64(666)
				packer.String("fields")
				packer.ArrayHeader(2)   // >> fields array
				packer.String("field1") //
				packer.String("field2") // << fields array
				packer.String("t_first")
				packer.Int64(10000)
				packer.String("qid")
				packer.Int64(777)
			},
			x: &success{tlast: -1, fields: []string{"field1", "field2"}, tfirst: 10000, qid: int64(777), num: 4},
		},
		{
			name: "Success discard/end of page response with more data",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(1)
				packer.String("has_more")
				packer.Bool(true)
			},
			x: &success{tlast: -1, tfirst: -1, hasMore: true, qid: -1, num: 1},
		},
		{
			name: "Success discard response with no more data",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("has_more")
				packer.Bool(false)
				packer.String("whatever") // >> Whatever array to ignore
				packer.ArrayHeader(2)     //
				packer.Int(1)             //
				packer.Int(2)             // << Whatever array
				packer.String("bookmark")
				packer.String("bm")
				packer.String("db")
				packer.String("sys")
			},
			x: &success{tlast: -1, tfirst: -1, bookmark: "bm", db: "sys", qid: -1, num: 4},
		},
		{
			name: "Success pull response, write with db",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("bookmark")
				packer.String("b")
				packer.String("t_last")
				packer.Int64(124)
				packer.String("type")
				packer.String("w")
				packer.String("db")
				packer.String("s")
			},
			x: &success{tlast: 124, tfirst: -1, bookmark: "b", qtype: db.StatementTypeWrite, db: "s", qid: -1, num: 4},
		},
		{
			name: "Success summary with plan",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("has_more")
				packer.Bool(false)
				packer.String("bookmark")
				packer.String("bm")
				packer.String("db")
				packer.String("sys")
				packer.String("plan") // Plan map
				packer.MapHeader(4)
				packer.String("operatorType")
				packer.String("opType")
				packer.String("identifiers") // array
				packer.ArrayHeader(2)
				packer.String("id1")
				packer.String("id2")
				packer.String("args") // map
				packer.MapHeader(1)
				packer.String("arg1")
				packer.Int(1001)
				packer.String("children") // array of maps
				packer.ArrayHeader(1)
				packer.MapHeader(2) // Another plan map
				packer.String("operatorType")
				packer.String("cop")
				packer.String("identifiers") // array
				packer.ArrayHeader(1)
				packer.String("cid")
			},
			x: &success{tlast: -1, tfirst: -1, bookmark: "bm", db: "sys", qid: -1, num: 4, plan: &db.Plan{
				Operator:    "opType",
				Arguments:   map[string]any{"arg1": int64(1001)},
				Identifiers: []string{"id1", "id2"},
				Children: []db.Plan{
					{Operator: "cop", Identifiers: []string{"cid"}, Children: []db.Plan{}},
				},
			}},
		},
		{
			name: "Success summary with profile",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("has_more")
				packer.Bool(false)
				packer.String("bookmark")
				packer.String("bm")
				packer.String("db")
				packer.String("sys")
				packer.String("profile") // Profile map
				packer.MapHeader(6)
				packer.String("operatorType")
				packer.String("opType")
				packer.String("dbHits")
				packer.Int(7)
				packer.String("rows")
				packer.Int(4)
				packer.String("identifiers") // array
				packer.ArrayHeader(2)
				packer.String("id1")
				packer.String("id2")
				packer.String("args") // map
				packer.MapHeader(1)
				packer.String("arg1")
				packer.Int(1001)
				packer.String("children") // array of maps
				packer.ArrayHeader(1)
				packer.MapHeader(4) // Another profile map
				packer.String("operatorType")
				packer.String("cop")
				packer.String("identifiers") // array
				packer.ArrayHeader(1)        //
				packer.String("cid")         // << array
				packer.String("dbHits")
				packer.Int(1)
				packer.String("rows")
				packer.Int(2)
			},
			x: &success{tlast: -1, tfirst: -1, bookmark: "bm", db: "sys", qid: -1, num: 4,
				profile: &db.ProfiledPlan{
					Operator:    "opType",
					Arguments:   map[string]any{"arg1": int64(1001)},
					Identifiers: []string{"id1", "id2"},
					Children: []db.ProfiledPlan{
						{Operator: "cop", Identifiers: []string{"cid"}, Children: []db.ProfiledPlan{}, DbHits: int64(1), Records: int64(2)},
					},
					DbHits:  int64(7),
					Records: int64(4),
				}},
		},
		{
			name: "Success summary with notifications",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("has_more")
				packer.Bool(false)
				packer.String("bookmark")
				packer.String("bm")
				packer.String("db")
				packer.String("sys")
				packer.String("notifications") // Array
				packer.ArrayHeader(2)
				packer.MapHeader(5) // Notification map
				packer.String("code")
				packer.String("c1")
				packer.String("title")
				packer.String("t1")
				packer.String("description")
				packer.String("d1")
				packer.String("severity")
				packer.String("s1")
				packer.String("position")
				packer.MapHeader(3)
				packer.String("offset")
				packer.Int(1)
				packer.String("line")
				packer.Int(2)
				packer.String("column")
				packer.Int(3)
				packer.MapHeader(4) // Notification map
				packer.String("code")
				packer.String("c2")
				packer.String("title")
				packer.String("t2")
				packer.String("description")
				packer.String("d2")
				packer.String("severity")
				packer.String("s2")
			},
			x: &success{tlast: -1, tfirst: -1, bookmark: "bm", db: "sys", qid: -1, num: 4,
				notifications: []db.Notification{
					{Code: "c1", Title: "t1", Description: "d1", Severity: "s1", Position: &db.InputPosition{Offset: 1, Line: 2, Column: 3}},
					{Code: "c2", Title: "t2", Description: "d2", Severity: "s2"},
				}},
		},
		{
			name: "Success pull response read no db",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(4)
				packer.String("bookmark")
				packer.String("b1")
				packer.String("t_last")
				packer.Int64(7)
				packer.String("type")
				packer.String("r")
				packer.String("has_more")
				packer.Bool(false)
			},
			x: &success{tlast: 7, tfirst: -1, bookmark: "b1", qtype: db.StatementTypeRead, qid: -1, num: 4},
		},
		{
			name: "Success route response",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(1)
				packer.String("rt")
				packer.MapHeader(3)
				packer.String("ttl")
				packer.Int(1001)
				packer.String("db")
				packer.String("dbname")
				packer.String("servers")
				packer.ArrayHeader(3)
				// Routes
				packer.MapHeader(2)
				packer.String("role")
				packer.String("ROUTE")
				packer.String("addresses")
				packer.ArrayHeader(2)
				packer.String("router1")
				packer.String("router2")
				// Readers
				packer.MapHeader(2)
				packer.String("role")
				packer.String("READ")
				packer.String("addresses")
				packer.ArrayHeader(3)
				packer.String("reader1")
				packer.String("reader2")
				packer.String("reader3")
				// Writers
				packer.MapHeader(2)
				packer.String("role")
				packer.String("WRITE")
				packer.String("addresses")
				packer.ArrayHeader(1)
				packer.String("writer1")
			},
			x: &success{tlast: -1, tfirst: -1, qid: -1, num: 1, routingTable: &idb.RoutingTable{
				TimeToLive:   1001,
				DatabaseName: "dbname",
				Routers:      []string{"router1", "router2"},
				Readers:      []string{"reader1", "reader2", "reader3"},
				Writers:      []string{"writer1"}}},
		},
		{
			name: "Success route response no database name(<4.4)",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(1)
				packer.String("rt")
				packer.MapHeader(2)
				packer.String("ttl")
				packer.Int(1001)
				packer.String("servers")
				packer.ArrayHeader(3)
				// Routes
				packer.MapHeader(2)
				packer.String("role")
				packer.String("ROUTE")
				packer.String("addresses")
				packer.ArrayHeader(2)
				packer.String("router1")
				packer.String("router2")
				// Readers
				packer.MapHeader(2)
				packer.String("role")
				packer.String("READ")
				packer.String("addresses")
				packer.ArrayHeader(3)
				packer.String("reader1")
				packer.String("reader2")
				packer.String("reader3")
				// Writers
				packer.MapHeader(2)
				packer.String("role")
				packer.String("WRITE")
				packer.String("addresses")
				packer.ArrayHeader(1)
				packer.String("writer1")
			},
			x: &success{tlast: -1, tfirst: -1, qid: -1, num: 1, routingTable: &idb.RoutingTable{
				TimeToLive: 1001,
				Routers:    []string{"router1", "router2"},
				Readers:    []string{"reader1", "reader2", "reader3"},
				Writers:    []string{"writer1"}}},
		},
		{
			name: "Success route response extras",
			build: func() {
				packer.StructHeader(byte(msgSuccess), 1)
				packer.MapHeader(2)
				packer.String("extra1")
				packer.ArrayHeader(2)
				packer.Int(1)
				packer.Int(2)
				packer.String("rt")
				packer.MapHeader(2)
				packer.String("ttl")
				packer.Int(1001)
				packer.String("servers")
				packer.ArrayHeader(1)
				// Routes
				packer.MapHeader(3)
				packer.String("extra2")
				packer.ArrayHeader(1)
				packer.String("extraval2")
				packer.String("role")
				packer.String("ROUTE")
				packer.String("addresses")
				packer.ArrayHeader(1)
				packer.String("router1")
			},
			x: &success{tlast: -1, tfirst: -1, qid: -1, num: 2, routingTable: &idb.RoutingTable{
				TimeToLive: 1001,
				Routers:    []string{"router1"}}},
		},
		{
			name: "Record of ints",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(5)
				packer.Int(1)
				packer.Int(2)
				packer.Int(3)
				packer.Int(4)
				packer.Int(5)
			},
			x: &db.Record{Values: []any{int64(1), int64(2), int64(3), int64(4), int64(5)}},
		},
		{
			name: "Record of spatials",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(2)
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(1)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.StructHeader('Y', 4) // Point3D
				packer.Int64(2)             //
				packer.Float64(0.123)       //
				packer.Float64(23.71)       //
				packer.Float64(3.712)       //
			},
			x: &db.Record{Values: []any{
				dbtype.Point2D{SpatialRefId: 1, X: 7.123, Y: 123.7},
				dbtype.Point3D{SpatialRefId: 2, X: 0.123, Y: 23.71, Z: 3.712},
			}},
		},
		{
			name: "Record of temporals",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(7)
				// Time
				packer.StructHeader('T', 2)
				packer.Int64(int64(time.Hour*1 + time.Minute*2 + time.Second*3 + 4))
				packer.Int64(6)
				// Local time
				packer.StructHeader('t', 1)
				packer.Int64(int64(time.Hour*1 + time.Minute*2 + time.Second*3 + 4))
				// Date
				packer.StructHeader('D', 1)
				packer.Int64(time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC).Unix() / (60 * 60 * 24))
				// Datetime, local
				packer.StructHeader('d', 2)
				t := time.Date(1999, 12, 31, 23, 59, 59, 1, time.UTC)
				packer.Int64(t.Unix())
				packer.Int64(t.UnixNano() - (t.Unix() * int64(time.Second)))
				// Datetime, named zone
				packer.StructHeader('f', 3)
				t = time.Date(1999, 12, 31, 23, 59, 59, 1, time.UTC)
				packer.Int64(t.Unix())
				packer.Int64(t.UnixNano() - (t.Unix() * int64(time.Second)))
				packer.String(zoneName)
				// Datetime, offset zone
				packer.StructHeader('F', 3)
				t = time.Date(1999, 12, 31, 23, 59, 59, 1, time.UTC)
				packer.Int64(t.Unix())
				packer.Int64(t.UnixNano() - (t.Unix() * int64(time.Second)))
				packer.Int(3)
				// Duration
				packer.StructHeader('E', 4)
				packer.Int64(12)
				packer.Int64(31)
				packer.Int64(59)
				packer.Int64(10001)
			},
			x: &db.Record{Values: []any{
				dbtype.Time(time.Date(0, 0, 0, 1, 2, 3, 4, time.FixedZone("Offset", 6))),
				dbtype.LocalTime(time.Date(0, 0, 0, 1, 2, 3, 4, time.Local)),
				dbtype.Date(time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC)),
				dbtype.LocalDateTime(time.Date(1999, 12, 31, 23, 59, 59, 1, time.Local)),
				time.Date(1999, 12, 31, 23, 59, 59, 1, timeZone),
				time.Date(1999, 12, 31, 23, 59, 59, 1, time.FixedZone("Offset", 3)),
				dbtype.Duration{Months: 12, Days: 31, Seconds: 59, Nanos: 10001},
			}},
		},
		{
			name: "Record with node",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('N', 3)
				packer.Int64(19000)
				packer.ArrayHeader(3)
				packer.String("lbl1")
				packer.String("lbl2")
				packer.String("lbl3")
				packer.MapHeader(2)
				packer.String("key1")
				packer.Int8(7)
				packer.String("key2")
				packer.ArrayHeader(2)
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(1)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(2)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
			},
			x: &db.Record{Values: []any{
				dbtype.Node{
					Id:        19000,
					ElementId: "19000",
					Labels:    []string{"lbl1", "lbl2", "lbl3"},
					Props: map[string]any{
						"key1": int64(7),
						"key2": []any{
							dbtype.Point2D{SpatialRefId: 1, X: 7.123, Y: 123.7},
							dbtype.Point2D{SpatialRefId: 2, X: 7.123, Y: 123.7},
						},
					}},
			}},
		},
		{
			name: "Record with relationship",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('R', 5)
				packer.Int64(19000)
				packer.Int64(19001)
				packer.Int64(1000)
				packer.String("lbl")
				packer.MapHeader(2)
				packer.String("key1")
				packer.Int8(7)
				packer.String("key2")
				packer.ArrayHeader(2)
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(1)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(2)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
			},
			x: &db.Record{Values: []any{
				dbtype.Relationship{
					Id:             19000,
					ElementId:      "19000",
					StartId:        19001,
					StartElementId: "19001",
					EndId:          1000,
					EndElementId:   "1000",
					Type:           "lbl",
					Props: map[string]any{
						"key1": int64(7),
						"key2": []any{
							dbtype.Point2D{SpatialRefId: 1, X: 7.123, Y: 123.7},
							dbtype.Point2D{SpatialRefId: 2, X: 7.123, Y: 123.7},
						},
					}},
			}},
		},
		{
			name: "Record with path",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('P', 3)
				// Two nodes
				packer.ArrayHeader(2)
				packer.StructHeader('N', 3) // Node 1
				packer.Int64(3)
				packer.ArrayHeader(1)
				packer.String("lbl1")
				packer.MapHeader(1)
				packer.String("key1")
				packer.Int8(7)
				packer.StructHeader('N', 3) // Node 2
				packer.Int64(7)
				packer.ArrayHeader(1)
				packer.String("lbl2")
				packer.MapHeader(1)
				packer.String("key2")
				packer.Int8(9)
				// Relation node
				packer.ArrayHeader(1)
				packer.StructHeader('r', 3)
				packer.Int(9)
				packer.String("x")
				packer.MapHeader(1)
				packer.String("akey")
				packer.String("aval")
				// Path
				packer.ArrayHeader(2)
				packer.Int(1)
				packer.Int(1)
			},
			x: &db.Record{Values: []any{
				dbtype.Path{
					Nodes: []dbtype.Node{
						{Id: 3, ElementId: "3", Labels: []string{"lbl1"}, Props: map[string]any{"key1": int64(7)}},
						{Id: 7, ElementId: "7", Labels: []string{"lbl2"}, Props: map[string]any{"key2": int64(9)}},
					},
					Relationships: []dbtype.Relationship{
						{Id: 9, ElementId: "9", StartId: 3, StartElementId: "3", EndId: 7, EndElementId: "7", Type: "x", Props: map[string]any{"akey": "aval"}},
					}},
			}},
		},
		{
			name:   "Record of UTC datetime with explicit offset with UTC support enabled",
			useUtc: true,
			build: func() {
				tz := time.FixedZone("Offset", 3)
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, tz)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// UTC Datetime with explicit time zone offset
				packer.StructHeader('I', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.Int(3)
			},
			x: &db.Record{Values: []any{
				time.Date(1999, 12, 31, 23, 59, 59, 1, time.FixedZone("Offset", 3)),
			}},
		},
		{
			name:   "Record of UTC datetime with explicit offset with UTC support disabled",
			useUtc: false,
			build: func() {
				tz := time.FixedZone("Offset", 3)
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, tz)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// UTC Datetime with explicit time zone offset
				packer.StructHeader('I', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.Int(3)
			},
			err: &db.ProtocolError{Err: "Received unknown struct tag: 73"},
		},
		{
			name:   "Record of legacy datetime with explicit offset with UTC support enabled",
			useUtc: true,
			build: func() {
				tz := time.FixedZone("Offset", 3)
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, tz)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// Datetime with explicit time zone offset
				packer.StructHeader('F', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.Int(3)
			},
			err: &db.ProtocolError{Err: "Received unknown struct tag: 70"},
		},
		{
			name:   "Record of UTC datetime with timezone name with UTC support enabled",
			useUtc: true,
			build: func() {
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, timeZone)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// UTC Datetime with time zone name
				packer.StructHeader('i', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.String(zoneName)
			},
			x: &db.Record{Values: []any{
				time.Date(1999, 12, 31, 23, 59, 59, 1, timeZone),
			}},
		},
		{
			name:   "Record of UTC datetime with timezone name with UTC support disabled",
			useUtc: false,
			build: func() {
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, timeZone)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// UTC Datetime with time zone name
				packer.StructHeader('i', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.String(zoneName)
			},
			err: &db.ProtocolError{Err: "Received unknown struct tag: 105"},
		},
		{
			name:   "Record of legacy datetime with timezone name with UTC support enabled",
			useUtc: true,
			build: func() {
				datetime := time.Date(1999, 12, 31, 23, 59, 59, 1, timeZone)
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				// UTC Datetime with time zone name
				packer.StructHeader('f', 3)
				packer.Int64(datetime.Unix())
				packer.Int64(datetime.UnixNano() - (datetime.Unix() * int64(time.Second)))
				packer.String(zoneName)
			},
			err: &db.ProtocolError{Err: "Received unknown struct tag: 102"},
		},
		{
			name:   "Record of UTC datetime with invalid timezone name with UTC support enabled",
			useUtc: true,
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('i', 3)
				packer.Int64(42)
				packer.Int64(42)
				packer.String("LA/Confidential")
			},
			x: &db.Record{Values: []any{
				&dbtype.InvalidValue{
					Message: "utcDateTimeNamedZone",
					Err:     fmt.Errorf("unknown time zone LA/Confidential"),
				},
			}},
		},
		{
			name: "Record of legacy datetime with invalid timezone name with UTC support disabled",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('f', 3)
				packer.Int64(42)
				packer.Int64(42)
				packer.String("LA/Confidential")
			},
			x: &db.Record{Values: []any{
				&dbtype.InvalidValue{
					Message: "dateTimeNamedZone",
					Err:     fmt.Errorf("unknown time zone LA/Confidential"),
				},
			}},
		},
	}

	// Shared among calls in real usage so we do the same while testing it.
	hydrator := hydrator{}
	for _, c := range cases {
		outer.Run(c.name, func(t *testing.T) {
			defer func() {
				hydrator.err = nil
			}()
			hydrator.useUtc = c.useUtc
			if (c.x != nil) == (c.err != nil) {
				t.Fatalf("test case needs to define either expected result or error (xor)")
			}
			packer.Begin([]byte{})
			c.build()
			buf, err := packer.End()
			if err != nil {
				panic("Build error")
			}
			x, err := hydrator.hydrate(buf)
			if c.err != nil {
				if !reflect.DeepEqual(err, c.err) {
					fmt.Printf("%+v", err)
					t.Fatalf("Expected:\n%+v\n != Actual: \n%+v\n", c.err, err)
				}
				return
			}
			if err != nil {
				panic(err)
			}
			if !reflect.DeepEqual(x, c.x) {
				fmt.Printf("%+v", hydrator.cachedSuccess.plan)
				t.Fatalf("Expected:\n%+v\n != Actual: \n%+v\n", c.x, x)
			}
		})
	}
}

func TestHydratorBolt5(outer *testing.T) {
	packer := packstream.Packer{}
	cases := []hydratorTestCase{
		{
			name: "Record with node with element ID",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('N', 4)
				packer.Int64(19000)
				packer.ArrayHeader(3)
				packer.String("lbl1")
				packer.String("lbl2")
				packer.String("lbl3")
				packer.MapHeader(2)
				packer.String("key1")
				packer.Int8(7)
				packer.String("key2")
				packer.ArrayHeader(2)
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(1)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(2)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.String("19091")
			},
			x: &db.Record{Values: []any{
				dbtype.Node{
					Id:        19000,
					ElementId: "19091",
					Labels:    []string{"lbl1", "lbl2", "lbl3"},
					Props: map[string]any{
						"key1": int64(7),
						"key2": []any{
							dbtype.Point2D{SpatialRefId: 1, X: 7.123, Y: 123.7},
							dbtype.Point2D{SpatialRefId: 2, X: 7.123, Y: 123.7},
						},
					}},
			}},
		},
		{
			name: "Record with relationship with element ID",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('R', 8)
				packer.Int64(19000)
				packer.Int64(19001)
				packer.Int64(1000)
				packer.String("lbl")
				packer.MapHeader(2)
				packer.String("key1")
				packer.Int8(7)
				packer.String("key2")
				packer.ArrayHeader(2)
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(1)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.StructHeader('X', 3) // Point2D
				packer.Int64(2)             //
				packer.Float64(7.123)       //
				packer.Float64(123.7)       //
				packer.String("19091")      // rel element ID
				packer.String("19191")      // start element ID
				packer.String("1001")       // end element ID
			},
			x: &db.Record{Values: []any{
				dbtype.Relationship{
					Id:             19000,
					ElementId:      "19091",
					StartId:        19001,
					StartElementId: "19191",
					EndId:          1000,
					EndElementId:   "1001",
					Type:           "lbl",
					Props: map[string]any{
						"key1": int64(7),
						"key2": []any{
							dbtype.Point2D{SpatialRefId: 1, X: 7.123, Y: 123.7},
							dbtype.Point2D{SpatialRefId: 2, X: 7.123, Y: 123.7},
						},
					}},
			}},
		},
		{
			name: "Record with path with element ID",
			build: func() {
				packer.StructHeader(byte(msgRecord), 1)
				packer.ArrayHeader(1)
				packer.StructHeader('P', 3)
				// Two nodes
				packer.ArrayHeader(2)
				packer.StructHeader('N', 4) // Node 1
				packer.Int64(3)
				packer.ArrayHeader(1)
				packer.String("lbl1")
				packer.MapHeader(1)
				packer.String("key1")
				packer.Int8(7)
				packer.String("33")         // node 1 element ID
				packer.StructHeader('N', 4) // Node 2
				packer.Int64(7)
				packer.ArrayHeader(1)
				packer.String("lbl2")
				packer.MapHeader(1)
				packer.String("key2")
				packer.Int8(9)
				packer.String("77") // node 2 element ID
				// Relation node
				packer.ArrayHeader(1)
				packer.StructHeader('r', 4)
				packer.Int(9)
				packer.String("x")
				packer.MapHeader(1)
				packer.String("akey")
				packer.String("aval")
				packer.String("99") // rel element ID
				// Path
				packer.ArrayHeader(2)
				packer.Int(1)
				packer.Int(1)
			},
			x: &db.Record{Values: []any{
				dbtype.Path{
					Nodes: []dbtype.Node{
						{Id: 3, ElementId: "33", Labels: []string{"lbl1"}, Props: map[string]any{"key1": int64(7)}},
						{Id: 7, ElementId: "77", Labels: []string{"lbl2"}, Props: map[string]any{"key2": int64(9)}},
					},
					Relationships: []dbtype.Relationship{
						{Id: 9, ElementId: "99", StartId: 3, StartElementId: "33", EndId: 7, EndElementId: "77", Type: "x", Props: map[string]any{"akey": "aval"}},
					}},
			}},
		},
	}

	hydrator := hydrator{boltMajor: 5, useUtc: true}
	for _, c := range cases {
		outer.Run(c.name, func(t *testing.T) {
			defer func() {
				hydrator.err = nil
			}()
			if (c.x != nil) == (c.err != nil) {
				t.Fatalf("test case needs to define either expected result or error (xor)")
			}
			packer.Begin([]byte{})
			c.build()
			buf, err := packer.End()
			if err != nil {
				panic("Build error")
			}
			x, err := hydrator.hydrate(buf)
			if c.err != nil {
				if !reflect.DeepEqual(err, c.err) {
					fmt.Printf("%+v", err)
					t.Fatalf("Expected:\n%+v\n != Actual: \n%+v\n", c.err, err)
				}
				return
			}
			if err != nil {
				panic(err)
			}
			if !reflect.DeepEqual(x, c.x) {
				fmt.Printf("%+v", hydrator.cachedSuccess.plan)
				t.Fatalf("Expected:\n%+v\n != Actual: \n%+v\n", c.x, x)
			}
		})
	}
}

func TestUtcDateTime(outer *testing.T) {
	// Thu Jun 16 2022 13:00:00 UTC
	secondsSinceEpoch := int64(1655384400)

	hydrator := &hydrator{useUtc: true}

	outer.Run("UTC Datetime with offset in seconds", func(t *testing.T) {
		offsetInSeconds := 2*60*60 + 30*60 // UTC+2h30
		bytes := recordOfUtcDateTimeWithOffset(t, secondsSinceEpoch, offsetInSeconds)

		rawRecord, err := hydrator.hydrate(bytes)

		if err != nil {
			t.Fatal(err)
		}
		record := rawRecord.(*db.Record)
		rawDatetime := record.Values[0]
		datetime := rawDatetime.(time.Time)
		_, offset := datetime.Zone()
		if offset != 2*60*60+30*60 {
			t.Fatalf("Expected offset of +2 hours (7200 seconds), got %d", offset)
		}
		year := datetime.Year()
		if year != 2022 {
			t.Errorf("Expected year 2022, got %d", year)
		}
		month := datetime.Month()
		if month != 6 {
			t.Errorf("Expected month of June (6), got %d", month)
		}
		day := datetime.Day()
		if day != 16 {
			t.Errorf("Expected day 16, got %d", day)
		}
		hour := datetime.Hour()
		if hour != 15 {
			t.Errorf("Expected hour 15, got %d", hour)
		}
		minutes := datetime.Minute()
		if minutes != 30 {
			t.Errorf("Expected minute 30, got %d", minutes)
		}
		seconds := datetime.Second()
		if seconds != 0 {
			t.Errorf("Expected second 0, got %d", seconds)
		}
		nanos := datetime.Nanosecond()
		if nanos != 0 {
			t.Errorf("Expected nanosecond 0, got %d", nanos)
		}
	})

	outer.Run("UTC Datetime with named timezone", func(t *testing.T) {
		timeZone := "Australia/Eucla" // UTC+8h45 in that point in time
		bytes := recordOfUtcDateTimeWithTimeZoneName(t, secondsSinceEpoch, timeZone)

		rawRecord, err := hydrator.hydrate(bytes)

		if err != nil {
			t.Fatal(err)
		}
		record := rawRecord.(*db.Record)
		rawDatetime := record.Values[0]
		datetime := rawDatetime.(time.Time)
		_, offset := datetime.Zone() // "Australia/Eucla" is normalized to sth else
		if offset != 8*60*60+45*60 {
			t.Fatalf("Expected +8h45 offset (31500 seconds), got %d", offset)
		}
		year := datetime.Year()
		if year != 2022 {
			t.Errorf("Expected year 2022, got %d", year)
		}
		month := datetime.Month()
		if month != 6 {
			t.Errorf("Expected month of June (6), got %d", month)
		}
		day := datetime.Day()
		if day != 16 {
			t.Errorf("Expected day 16, got %d", day)
		}
		hour := datetime.Hour()
		if hour != 21 {
			t.Errorf("Expected hour 21, got %d", hour)
		}
		minutes := datetime.Minute()
		if minutes != 45 {
			t.Errorf("Expected minute 45, got %d", minutes)
		}
		seconds := datetime.Second()
		if seconds != 0 {
			t.Errorf("Expected second 0, got %d", seconds)
		}
		nanos := datetime.Nanosecond()
		if nanos != 0 {
			t.Errorf("Expected nanosecond 0, got %d", nanos)
		}
	})
}

func recordOfUtcDateTimeWithOffset(t *testing.T, secondsSinceEpoch int64, utcOffsetInSeconds int) []byte {
	packer := packstream.Packer{}
	packer.Begin([]byte{})
	packer.StructHeader(msgRecord, 1)
	packer.ArrayHeader(1)
	packer.StructHeader('I', 3)
	packer.Int64(secondsSinceEpoch)
	packer.Int64(0)
	packer.Int(utcOffsetInSeconds)
	result, err := packer.End()
	if err != nil {
		t.Fatal("Build error")
	}
	return result
}

func recordOfUtcDateTimeWithTimeZoneName(t *testing.T, secondsSinceEpoch int64, tzName string) []byte {
	packer := packstream.Packer{}
	packer.Begin([]byte{})
	packer.StructHeader(msgRecord, 1)
	packer.ArrayHeader(1)
	packer.StructHeader('i', 3)
	packer.Int64(secondsSinceEpoch)
	packer.Int64(0)
	packer.String(tzName)
	result, err := packer.End()
	if err != nil {
		t.Fatal("Build error")
	}
	return result
}
