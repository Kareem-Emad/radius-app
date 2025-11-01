package accounting

import (
	"fmt"
	"net"
	"testing"
	"time"

	"dni/pkg/datastore"
	"dni/pkg/stream"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redismock/v8"
	"go.llib.dev/testcase/clock"
	"go.llib.dev/testcase/clock/timecop"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

// mockResponseWriter implements radius.ResponseWriter for testing
type mockResponseWriter struct {
	response *radius.Packet
	written  bool
}

func (m *mockResponseWriter) Write(response *radius.Packet) error {
	m.response = response
	m.written = true
	return nil
}

func createAccountingRequest(username, sessionID string, statusType rfc2866.AcctStatusType) *radius.Request {
	secret := []byte("testing123")
	packet := radius.New(radius.CodeAccountingRequest, secret)

	rfc2865.UserName_SetString(packet, username)
	rfc2866.AcctSessionID_SetString(packet, sessionID)
	rfc2866.AcctStatusType_Set(packet, statusType)
	rfc2865.NASIPAddress_Set(packet, net.IPv4(192, 168, 1, 1))
	rfc2865.NASPort_Set(packet, 1234)
	rfc2865.FramedIPAddress_Set(packet, net.IPv4(10, 0, 0, 1))
	rfc2865.CallingStationID_SetString(packet, "00:11:22:33:44:55")
	rfc2865.CalledStationID_SetString(packet, "00:aa:bb:cc:dd:ee")

	if statusType == rfc2866.AcctStatusType_Value_Stop {
		rfc2866.AcctInputOctets_Set(packet, 1024)
		rfc2866.AcctOutputOctets_Set(packet, 2048)
		rfc2866.AcctSessionTime_Set(packet, 3600)
	}

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:1813")

	return &radius.Request{
		Packet:     packet,
		RemoteAddr: addr,
	}
}

func TestHandler_Handle_StartPacket(t *testing.T) {
	// freezing time to avoid flaky tests
	timecop.Travel(t, time.Unix(1, 0), timecop.Freeze)

	redisClient, mock := redismock.NewClientMock()
	defer redisClient.Close()

	dataStore := datastore.NewRedisStore(redisClient)
	streamClient := stream.NewRedisStream(redisClient)

	handler := NewHandler(dataStore, streamClient, time.Hour)

	mock.ExpectHMSet(
		"radius:acct:testuser:session123",
		"username", "testuser",
		"nas_ip_address", "192.168.1.1",
		"nas_port", "1234",
		"acct_status_type", `1`,
		"acct_session_id", "session123",
		"framed_ip_address", "10.0.0.1",
		"calling_station_id", "00:11:22:33:44:55",
		"called_station_id", "00:aa:bb:cc:dd:ee",
		"packet_type", "Accounting-Request",
		"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
	).SetVal(true)

	mock.ExpectExpire("radius:acct:testuser:session123", time.Hour).SetVal(true)

	mock.ExpectXAdd(&redis.XAddArgs{
		Stream: "radius:updates:testuser",
		Values: []interface{}{
			"key", "radius:acct:testuser:session123",
			"timestamp", clock.Now().Unix(),
			"username", "testuser",
		},
	}).SetVal("1-0")

	request := createAccountingRequest("testuser", "session123", rfc2866.AcctStatusType_Value_Start)
	responseWriter := &mockResponseWriter{}

	handler.Handle(responseWriter, request)

	if !responseWriter.written {
		t.Error("Response was not written")
	}
	if responseWriter.response.Code != radius.CodeAccountingResponse {
		t.Errorf("Expected AccountingResponse code, got %v", responseWriter.response.Code)
	}

}
