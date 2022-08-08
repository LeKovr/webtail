package webtail_test

import (
	"os"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LeKovr/webtail"
)

const (
	newline = "\n"

	// File for testing
	RootFile = "file1.log"
)

type ServerSuite struct {
	suite.Suite
	cfg webtail.Config
}

func (ss *ServerSuite) SetupSuite() {
	// Fill config with default values
	p := flags.NewParser(&ss.cfg, flags.Default)
	_, err := p.ParseArgs([]string{})
	require.NoError(ss.T(), err)

	ss.cfg.Root = "testdata"
	ss.cfg.Trace = true // testing.Verbose()
	ss.cfg.Bytes = 20
}

func (ss *ServerSuite) TearDownSuite() {
	os.Remove(ss.cfg.Root + "/" + RootFile)
}

func (ss *ServerSuite) TestSimpleCommands() {
	wtc, err := NewWebTailClient(ss.T(), &ss.cfg)
	require.NoError(ss.T(), err)
	defer wtc.Close()
	tests := []struct {
		name string
		cmd  *webtail.InMessage
		want []string
	}{
		{
			name: "Get current trace",
			cmd:  &webtail.InMessage{Type: "trace"},
			want: []string{`{"enabled":true,"type":"trace"}`},
		}, {
			name: "Disable trace",
			cmd:  &webtail.InMessage{Type: "trace", Channel: "off"},
			want: []string{`{"enabled":false,"type":"trace"}`},
		}, {
			name: "Enable trace",
			cmd:  &webtail.InMessage{Type: "trace", Channel: "on"},
			want: []string{`{"enabled":true,"type":"trace"}`},
		}, {
			name: "Try to unsubscribe when not subscribed",
			cmd:  &webtail.InMessage{Type: "detach"},
			want: []string{`{"data":"not subscribed","type":"error"}`},
		}, {
			name: "Try to subscribe to unexistent channel",
			cmd:  &webtail.InMessage{Type: "attach", Channel: ".notexists"},
			want: []string{`{"channel":".notexists","data":"unknown channel","type":"error"}`},
		}, {
			name: "Try to unsubscribe from unexistent channel",
			cmd:  &webtail.InMessage{Type: "detach", Channel: ".notexists"},
			want: []string{`{"channel":".notexists","data":"unknown channel","type":"error"}`},
		}, {
			name: "Subscribe on file",
			cmd:  &webtail.InMessage{Type: "attach", Channel: "subdir/another.log"},
			want: []string{`{"channel":"subdir/another.log","data":"success","type":"attach"}`},
		}, {
			name: "Set subscriber count",
			cmd:  &webtail.InMessage{Type: "stats"},
			want: []string{`{"data":{"subdir/another.log":1},"type":"stats"}`},
		}, {
			name: "Try to subscribe again",
			cmd:  &webtail.InMessage{Type: "attach", Channel: "subdir/another.log"},
			want: []string{`{"channel":"subdir/another.log","data":"attached already","type":"error"}`},
		},
	}
	go wtc.Listener(len(tests) + 1)

	for _, tt := range tests {
		got := wtc.Call(tt.cmd, len(tt.want), false)
		require.Equal(ss.T(), tt.want, got)
	}

	want := []string{`{"data":"parse error","type":"error"}`}
	err = wtc.ws.WriteJSON(`bad"data`)
	require.NoError(wtc.t, err, "Incorrect request")
	got := wtc.Receive(len(want), false)
	require.Equal(ss.T(), want, got)
}

func (ss *ServerSuite) TestTail() {
	wtc, err := NewWebTailClient(ss.T(), &ss.cfg)
	require.NoError(ss.T(), err)
	defer wtc.Close()
	go wtc.Listener(12)

	want := []string{`{"data":"success","type":"attach"}`}
	got := wtc.Call(&webtail.InMessage{Type: "attach"}, len(want), false)
	require.Equal(ss.T(), want, got)
	wtc.WaitSync(1) // wait for index attach

	testFile := ss.cfg.Root + "/" + RootFile
	f, err := os.Create(testFile)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row zero\ntest log row one\ntest log row two\n")
	require.NoError(ss.T(), err)
	f.Close()

	wtc.WaitSync(2) // wait for RootFile create & write
	want = []string{
		`{"data":{"name":"file.log","size":28},"type":"index"}`,
		`{"data":{"name":"file1.log","size":52},"type":"index"}`,
		`{"data":{"name":"file1.log","size":52},"type":"index"}`,
		`{"data":{"name":"subdir/another.log","size":22},"type":"index"}`,
	}
	got = wtc.Receive(len(want), true)
	require.Equal(ss.T(), want, got)

	want = []string{
		`{"channel":"file1.log","data":"success","type":"attach"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "attach", Channel: RootFile}, len(want), false)
	require.Equal(ss.T(), want, got)

	f, err = os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(ss.T(), err)
	_, err = f.WriteString("test log row three\n")
	require.NoError(ss.T(), err)
	f.Close()
	wtc.WaitSync(1) // wait for RootFile update

	want = []string{
		`{"channel":"file1.log","data":"success","type":"detach"}`,
		`{"channel":"file1.log","data":"test log row three","type":"log"}`,
		`{"channel":"file1.log","data":"test log row two","type":"log"}`,
		`{"data":{"name":"file1.log","size":71},"type":"index"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "detach", Channel: RootFile}, len(want), true)
	require.Equal(ss.T(), want, got)

	os.Remove(testFile)
	wtc.WaitSync(1) // wait for RootFile delete

	want = []string{
		`{"data":{"deleted":true,"name":"file1.log","size":0},"type":"index"}`,
		`{"data":"success","type":"detach"}`,
	}
	got = wtc.Call(&webtail.InMessage{Type: "detach"}, len(want), false)
	require.Equal(ss.T(), want, got)
}

func TestSuite(t *testing.T) {
	myTest := &ServerSuite{}
	suite.Run(t, myTest)
}
