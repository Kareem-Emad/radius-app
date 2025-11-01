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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled Redis expectations: %s", err)
	}
}

func TestHandler_Handle_Scenarios(t *testing.T) {
	// freezing time to avoid flaky tests
	timecop.Travel(t, time.Unix(1, 0), timecop.Freeze)

	tests := []struct {
		name           string
		username       string
		sessionID      string
		statusType     rfc2866.AcctStatusType
		setupMock      func(mock redismock.ClientMock)
		expectedResult bool // true = success, false = should handle errors gracefully
	}{
		{
			name:       "START packet - successful flow",
			username:   "testuser",
			sessionID:  "session123",
			statusType: rfc2866.AcctStatusType_Value_Start,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session123",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "1",
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
			},
			expectedResult: true,
		},
		{
			name:       "STOP packet - successful flow",
			username:   "testuser",
			sessionID:  "session456",
			statusType: rfc2866.AcctStatusType_Value_Stop,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session456",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "2", // Stop = 2
					"acct_session_id", "session456",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
					"acct_input_octets", "1024",
					"acct_output_octets", "2048",
					"acct_session_time", "3600",
				).SetVal(true)

				mock.ExpectExpire("radius:acct:testuser:session456", time.Hour).SetVal(true)

				mock.ExpectXAdd(&redis.XAddArgs{
					Stream: "radius:updates:testuser",
					Values: []interface{}{
						"key", "radius:acct:testuser:session456",
						"timestamp", clock.Now().Unix(),
						"username", "testuser",
					},
				}).SetVal("1-0")
			},
			expectedResult: true,
		},
		{
			name:       "START packet - datastore HMSET failure",
			username:   "testuser",
			sessionID:  "session789",
			statusType: rfc2866.AcctStatusType_Value_Start,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session789",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "1",
					"acct_session_id", "session789",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
				).SetErr(fmt.Errorf("Redis connection failed"))
			},
			expectedResult: false,
		},
		{
			name:       "START packet - datastore EXPIRE failure",
			username:   "testuser",
			sessionID:  "session101",
			statusType: rfc2866.AcctStatusType_Value_Start,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session101",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "1",
					"acct_session_id", "session101",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
				).SetVal(true)
				mock.ExpectExpire("radius:acct:testuser:session101", time.Hour).SetErr(fmt.Errorf("TTL setting failed"))
			},
			expectedResult: false,
		},
		{
			name:       "START packet - stream XADD failure",
			username:   "testuser",
			sessionID:  "session202",
			statusType: rfc2866.AcctStatusType_Value_Start,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session202",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "1",
					"acct_session_id", "session202",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
				).SetVal(true)
				mock.ExpectExpire("radius:acct:testuser:session202", time.Hour).SetVal(true)
				mock.ExpectXAdd(&redis.XAddArgs{
					Stream: "radius:updates:testuser",
					Values: []interface{}{
						"key", "radius:acct:testuser:session202",
						"timestamp", clock.Now().Unix(),
						"username", "testuser",
					},
				}).SetErr(fmt.Errorf("Stream publish failed"))
			},
			expectedResult: false,
		},
		{
			name:       "STOP packet - datastore failure",
			username:   "testuser",
			sessionID:  "session303",
			statusType: rfc2866.AcctStatusType_Value_Stop,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session303",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "2", // Stop = 2
					"acct_session_id", "session303",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
					"acct_input_octets", "1024",
					"acct_output_octets", "2048",
					"acct_session_time", "3600",
				).SetErr(fmt.Errorf("Database unavailable"))
			},
			expectedResult: false,
		},
		{
			name:       "STOP packet - stream failure",
			username:   "testuser",
			sessionID:  "session404",
			statusType: rfc2866.AcctStatusType_Value_Stop,
			setupMock: func(mock redismock.ClientMock) {
				mock.ExpectHMSet(
					"radius:acct:testuser:session404",
					"username", "testuser",
					"nas_ip_address", "192.168.1.1",
					"nas_port", "1234",
					"acct_status_type", "2", // Stop = 2
					"acct_session_id", "session404",
					"framed_ip_address", "10.0.0.1",
					"calling_station_id", "00:11:22:33:44:55",
					"called_station_id", "00:aa:bb:cc:dd:ee",
					"packet_type", "Accounting-Request",
					"timestamp", fmt.Sprintf("%d", clock.Now().Unix()),
					"acct_input_octets", "1024",
					"acct_output_octets", "2048",
					"acct_session_time", "3600",
				).SetVal(true)
				mock.ExpectExpire("radius:acct:testuser:session404", time.Hour).SetVal(true)
				mock.ExpectXAdd(&redis.XAddArgs{
					Stream: "radius:updates:testuser",
					Values: []interface{}{
						"key", "radius:acct:testuser:session404",
						"timestamp", clock.Now().Unix(),
						"username", "testuser",
					},
				}).SetErr(fmt.Errorf("Stream connection lost"))
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisClient, mock := redismock.NewClientMock()
			defer redisClient.Close()

			dataStore := datastore.NewRedisStore(redisClient)
			streamClient := stream.NewRedisStream(redisClient)
			handler := NewHandler(dataStore, streamClient, time.Hour)

			tt.setupMock(mock)

			request := createAccountingRequest(tt.username, tt.sessionID, tt.statusType)
			responseWriter := &mockResponseWriter{}

			handler.Handle(responseWriter, request)

			if !responseWriter.written {
				t.Error("Response was not written - handler should always respond per RADIUS protocol")
			}

			if responseWriter.response.Code != radius.CodeAccountingResponse {
				t.Errorf("Expected AccountingResponse code, got %v", responseWriter.response.Code)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("There were unfulfilled Redis expectations: %s", err)
			}
		})
	}
}
