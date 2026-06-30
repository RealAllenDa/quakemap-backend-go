package dmdata

import (
	"encoding/json"
	"testing"
)

func TestStartSocketSuccess(t *testing.T) {
	raw := []byte(`{
    "responseId": "83c36173ceaf9e44",
    "responseTime": "2021-04-01T00:00:00.000Z",
    "status": "ok",
    "ticket": "Tik....",
    "websocket": {
        "id": 0,
        "url": "wss://ws003.api.dmdata.jp/v2/websocket?ticket=Tik....",
        "protocol": [
            "dmdata.v2"
        ],
        "expiration": 300
    },
    "classifications": [
        "telegram.weather",
        "telegram.earthquake"
    ],
    "test": "no",
    "types": [
        "VXSE51",
        "VXSE52",
        "VXSE53",
        "VPWW54"
    ],
    "formats": [
        "xml",
        "a/n",
        "binary"
    ],
    "appName": "Application Test"
}`)
	var response SocketResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.WebSocket.Protocol) != 1 || response.WebSocket.Protocol[0] != "dmdata.v2" {
		t.Fatalf("unexpected protocol: %#v", response.WebSocket.Protocol)
	}
}

func TestStartSocketFail(t *testing.T) {
	raw := []byte(`{
    "responseId": "66d23c0cede77d82",
    "responseTime": "2021-04-01T00:00:00.000Z",
    "status": "error",
    "error": {
        "message": "The body of the request is not json.",
        "code": 400
    }
}`)
	var response SocketResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
}

func TestSocketPing(t *testing.T) {
	raw := []byte(`{
    "type": "ping",
    "pingId": "012345"
}`)
	var response SocketPing
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
}

func TestSocketStart(t *testing.T) {
	raw := []byte(`{
    "type": "start",
    "socketId": 1,
    "classifications": [
        "telegram.weather"
    ],
    "types": ["VPWW54"],
    "test": "including",
    "formats": ["xml", "a/n", "binary"],
    "appName": null,
    "time": "2020-01-01T00:00:00.000Z"
}`)
	var response SocketStart
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
}

func TestSocketError(t *testing.T) {
	raw := []byte(`{
    "type": "error",
    "error": "Server error.",
    "code": 4503,
    "close": true
}`)
	var response SocketError
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
}

func TestSocketData(t *testing.T) {
	raw := []byte(`{
  "type": "data",
  "version": "2.0",
  "id": "71f24e19332c218104bd1d3794be8ac6594d07240d34c36db49f5812a8dac754579c92292a8d0e9a8f8da22085e6faae",
  "classification": "eew.forecast",
  "passing": [
    {
      "name": "socket-01",
      "time": "2023-01-17T07:38:43.455Z"
    },
    {
      "name": "ires-13",
      "time": "2023-01-17T07:38:43.460Z"
    },
    {
      "name": "websocket-03",
      "time": "2023-01-17T07:38:43.462Z"
    }
  ],
  "head": {
    "type": "VXSE45",
    "author": "RJTD",
    "time": "2023-01-17T07:38:00.000Z",
    "designation": null,
    "test": false,
    "xml": true
  },
  "xmlReport": {
    "control": {
      "title": "緊急地震速報（予報）",
      "dateTime": "2023-01-17T07:38:43Z",
      "status": "通常",
      "editorialOffice": "気象庁本庁",
      "publishingOffice": "気象庁"
    },
    "head": {
      "title": "緊急地震速報（予報）",
      "reportDateTime": "2023-01-17T16:38:43+09:00",
      "targetDateTime": "2023-01-17T16:38:43+09:00",
      "eventId": "20230117163800",
      "serial": "5",
      "infoType": "発表",
      "infoKind": "緊急地震速報",
      "infoKindVersion": "1.0_0",
      "headline": null
    }
  },
  "format": "xml",
  "compression": "gzip",
  "encoding": "base64",
  "body": "H4sIAAAAAAAAAJVVS2/bRhC++1cIvBrWknr4IawYOI6DGE3sIlZ76CVYi2tpbb66pAzpJipImqZJ4SRuDQcO0qBN4UPixigMqa7SP8OQik76Cx2SEvWI0DoCQQ5nvlnu983sCF+pampij3KLGXpekJKikKB60VCYXsoLXxWuzy0KV+QZfJuaBrcTANatvFC2bTOHELwld5lVNirJkpHcMdGORsAnISEC5na06iXA8kwigVcM3eaGKuMCs1Uqd5oP/fpr7/hd9/i7bv2l98tZr/3gw8WD0PgeowiFrxGbFphG5ZSYSs+J0py0UBAXcunFXCb9DUZxGG/axK5Ycrf+3Gu1MOq/4lWF2QZnRN3Y3mZFKvvvDj+evfIuHP/4DdwxmgTgLytbKrAAeSZTMPokhtGAVkDxBiXKZRVk+rbBNWJDVa4Si1mBTJ8nTVSxaQJJ85FAs+JSThQxmkDiAuEleqnMCSRe3aO6vXYtTBElaUGaTy8GuIEfrwGtQs0EEkcXH1+dYBQ78CYNZJazUJzICsFfMF2ZwjhKDIMx7OuoiWXo4TviEDBwh/KrTA8I0qqNoDhDT2iGRbpqKLXLFmkLsMiizNIM1SjVxvr+Dt36/wWoSjVQZljhddjZsrLHLIPXZNd55jqn/t17QNhtPHUbDdf51XV+d+vOtB547h/XO+eNAOycuM6h6/zjOkduvYHR2LJ4lXC7/G2F7ALxDc5KTJ9W54VcNjuo8wgKL3PO9og6vTVEcZAyCsM3aqZRBKKUB/mUAFEC/g+th17zBbDwf/7Le3TYvQsNEQZgGig0YUNf5AVg6P12CiBv/7EgS6lscKgUwEQi51YMg8O0gh5MKNQqcmba4SiDBTvNP3rtg/D6sdfe9y5Aurr/4qxz/qjXfhL6n4ahg36oCdL9BEYY3e+9/6H3/rGQUGBWaHnBP3wNU8FvvYG9dP78W5BnM5mkOCtlUsnMnCTCD2H0yaaCg6hUijTkBXvyz5td5xlcYAdHL471cSPMOy/fdlqnnYOTcQmWRCk9yIyUuEl0ZYNvgq7e/XvdI5hvQ09QieCxXCxWOCkG5TdZVIsEJ/puXpgXQiOVF5YEeZ2sw3EdIGDAUtMux8AoHPpkfIuUdGZXFLpC1GJFDWdVH5ntI6dBoPYVbYvyje2p0Qy0wH/FgU/MBI32VV/5OKkv4q0dYbwveu23g454IsiZpBQXLU6FlUfPyBp8QLeYDV+8bnBaJJY9tCAIL9zQZChK+MQFI7DhDo5R1LJpUl1h1UC6KnhWykQvUVkMdBp5H4vepsQC1hOYvhe0iJdEw62hkQ2jYJ7B/3d/yMsz/wL+4z8U8QcAAA=="
}
`)
	var response SocketError
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
}
