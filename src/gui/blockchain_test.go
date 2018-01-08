package gui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/require"

	"crypto/rand"

	"math"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"

	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/testutil"
	"github.com/skycoin/skycoin/src/visor"
)

func (gw *FakeGateway) GetBlockByHash(hash cipher.SHA256) (block coin.SignedBlock, ok bool) {
	args := gw.Called(hash)
	return args.Get(0).(coin.SignedBlock), args.Bool(1)
}

// GetBlockBySeq returns blcok by seq
func (gw *FakeGateway) GetBlockBySeq(seq uint64) (block coin.SignedBlock, ok bool) {
	args := gw.Called(seq)
	return args.Get(0).(coin.SignedBlock), args.Bool(1)
}

func (gw *FakeGateway) GetLastBlocks(num uint64) (*visor.ReadableBlocks, error) {
	args := gw.Called(num)
	return args.Get(0).(*visor.ReadableBlocks), args.Error(1)
}

func randBytes(t *testing.T, n int) []byte {
	b := make([]byte, n)
	x, err := rand.Read(b)
	assert.Equal(t, n, x)
	assert.Nil(t, err)
	return b
}

func makeBadBlock(t *testing.T) *coin.Block {
	genPublic, _ := cipher.GenerateKeyPair()
	genAddress := cipher.AddressFromPubKey(genPublic)
	var genCoins uint64 = 1000e6
	var genTime uint64 = 1000
	now := genTime + 100
	preBlock, err := coin.NewGenesisBlock(genAddress, genCoins, genTime)
	require.NoError(t, err)
	uxHash := testutil.RandSHA256(t)
	tx := coin.Transaction{}
	tx.PushOutput(genAddress, math.MaxInt64+1, 255)
	b, err := coin.NewBlock(*preBlock, now, uxHash, coin.Transactions{tx}, feeCalc)
	require.NoError(t, err)
	return b
}
func feeCalc(t *coin.Transaction) (uint64, error) {
	return 0, nil
}

func TestGetBlock(t *testing.T) {

	badBlock := makeBadBlock(t)

	h := cipher.SHA256{}
	h.Set(randBytes(t, 32))
	validHashString := h.Hex()
	validSHA256, err := cipher.SHA256FromHex(validHashString)
	require.NoError(t, err)
	tt := []struct {
		name                        string
		method                      string
		url                         string
		status                      int
		err                         string
		hash                        string
		sha256                      cipher.SHA256
		seqStr                      string
		seq                         uint64
		gatewayGetBlockByHashResult coin.SignedBlock
		gatewayGetBlockByHashExists bool
		gatewayGetBlockBySeqResult  coin.SignedBlock
		gatewayGetBlockBySeqExists  bool
		response                    *visor.ReadableBlock
	}{
		{
			"405",
			http.MethodPost,
			"/block",
			http.StatusMethodNotAllowed,
			"405 Method Not Allowed",
			"hashExample",
			cipher.SHA256{},
			"sequence",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"400 - no seq and hash",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - should specify one filter, hash or seq",
			"",
			cipher.SHA256{},
			"",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"400 - seq and hash",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - should only specify one filter, hash or seq",
			"hash",
			cipher.SHA256{},
			"seq",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"400 - hash: encoding/hex err invalid byte: U+0068 'h'",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - encoding/hex: invalid byte: U+0068 'h'",
			"hash",
			cipher.SHA256{},
			"",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"400 - hash: encoding/hex: odd length hex string",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - encoding/hex: odd length hex string",
			"1234abc",
			cipher.SHA256{},
			"",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"400 - hash: Invalid hex length",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - Invalid hex length",
			"1234abcd",
			cipher.SHA256{},
			"",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"404 - hash",
			http.MethodGet,
			"/block",
			http.StatusNotFound,
			"404 Not Found",
			validHashString,
			validSHA256,
			"",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{
				Head: visor.ReadableBlockHeader{
					BkSeq:             0x0,
					BlockHash:         "7b8ec8dd836b564f0c85ad088fc744de820345204e154bc1503e04e9d6fdd9f1",
					PreviousBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
					Time:              0x0,
					Fee:               0x0,
					Version:           0x0,
					BodyHash:          "0000000000000000000000000000000000000000000000000000000000000000",
				},
				Body: visor.ReadableBlockBody{
					Transactions: []visor.ReadableTransaction{},
				},
			},
		},
		{
			"200 - hash",
			http.MethodGet,
			"/block",
			http.StatusOK,
			"",
			validHashString,
			validSHA256,
			"",
			0,
			coin.SignedBlock{},
			true,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{
				Head: visor.ReadableBlockHeader{
					BkSeq:             0x0,
					BlockHash:         "7b8ec8dd836b564f0c85ad088fc744de820345204e154bc1503e04e9d6fdd9f1",
					PreviousBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
					Time:              0x0,
					Fee:               0x0,
					Version:           0x0,
					BodyHash:          "0000000000000000000000000000000000000000000000000000000000000000",
				},
				Body: visor.ReadableBlockBody{
					Transactions: []visor.ReadableTransaction{},
				},
			},
		},
		{
			"400 - seq: ",
			http.MethodGet,
			"/block",
			http.StatusBadRequest,
			"400 Bad Request - strconv.ParseUint: parsing \"seq\": invalid syntax",
			"",
			cipher.SHA256{},
			"seq",
			0,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"404 - seq",
			http.MethodGet,
			"/block",
			http.StatusNotFound,
			"404 Not Found",
			"",
			cipher.SHA256{},
			"1",
			1,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			false,
			&visor.ReadableBlock{},
		},
		{
			"500 - NewReadableBlock error",
			http.MethodGet,
			"/block",
			http.StatusInternalServerError,
			"500 Internal Server Error",
			"",
			cipher.SHA256{},
			"1",
			1,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{
				Block: *badBlock,
			},
			true,
			&visor.ReadableBlock{},
		},
		{
			"200 - seq",
			http.MethodGet,
			"/block",
			http.StatusOK,
			"",
			"",
			cipher.SHA256{},
			"1",
			1,
			coin.SignedBlock{},
			false,
			coin.SignedBlock{},
			true,
			&visor.ReadableBlock{
				Head: visor.ReadableBlockHeader{
					BkSeq:             0x0,
					BlockHash:         "7b8ec8dd836b564f0c85ad088fc744de820345204e154bc1503e04e9d6fdd9f1",
					PreviousBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
					Time:              0x0,
					Fee:               0x0,
					Version:           0x0,
					BodyHash:          "0000000000000000000000000000000000000000000000000000000000000000",
				},
				Body: visor.ReadableBlockBody{
					Transactions: []visor.ReadableTransaction{},
				},
			},
		},
	}

	// Truncated hex hash
	//h := cipher.SumSHA256(randBytes(t, 128))

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gateway := &FakeGateway{
				t: t,
			}

			gateway.On("GetBlockByHash", tc.sha256).Return(tc.gatewayGetBlockByHashResult, tc.gatewayGetBlockByHashExists)
			gateway.On("GetBlockBySeq", tc.seq).Return(tc.gatewayGetBlockBySeqResult, tc.gatewayGetBlockBySeqExists)
			v := url.Values{}
			var url = tc.url
			if tc.hash != "" {
				v.Add("hash", tc.hash)
			}
			if tc.seqStr != "" {
				v.Add("seq", tc.seqStr)
			}
			if len(v) > 0 {
				url += "?" + v.Encode()
			}

			req, err := http.NewRequest(tc.method, url, nil)
			require.NoError(t, err)
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(getBlock(gateway))

			handler.ServeHTTP(rr, req)

			status := rr.Code
			require.Equal(t, tc.status, status, "case: %s, handler returned wrong status code: got `%v` want `%v`", tc.name, status, tc.status)

			if status != http.StatusOK {
				require.Equal(t, tc.err, strings.TrimSpace(rr.Body.String()), "case: %s, handler returned wrong error message: got `%v`| %s, want `%v`",
					tc.name, strings.TrimSpace(rr.Body.String()), status, tc.err)
			} else {
				var msg *visor.ReadableBlock
				err := json.Unmarshal(rr.Body.Bytes(), &msg)
				require.NoError(t, err)
				require.Equal(t, tc.response, msg)
			}
		})
	}
}

func TestGetLastBlocks(t *testing.T) {
	type httpBody struct {
		Num string
	}
	tt := []struct {
		name                       string
		method                     string
		url                        string
		status                     int
		err                        string
		body                       httpBody
		num                        uint64
		gatewayGetLastBlocksResult *visor.ReadableBlocks
		gatewayGetLastBlocksError  error
		response                   *visor.ReadableBlocks
	}{
		{
			"405",
			http.MethodPost,
			"/last_blocks",
			http.StatusMethodNotAllowed,
			"405 Method Not Allowed",
			httpBody{
				Num: "1",
			},
			1,
			&visor.ReadableBlocks{},
			nil,
			&visor.ReadableBlocks{},
		},
		{
			"400 - empty num value",
			http.MethodGet,
			"/last_blocks",
			http.StatusBadRequest,
			"400 Bad Request - Param: num is empty",
			httpBody{},
			1,
			&visor.ReadableBlocks{},
			nil,
			&visor.ReadableBlocks{},
		},
		{
			"400 - bad num value",
			http.MethodGet,
			"/last_blocks",
			http.StatusBadRequest,
			"400 Bad Request - strconv.ParseUint: parsing \"badNumValue\": invalid syntax",
			httpBody{
				Num: "badNumValue",
			},
			1,
			&visor.ReadableBlocks{},
			nil,
			&visor.ReadableBlocks{},
		},
		{
			"400 - gatewayGetLastBlocksError",
			http.MethodGet,
			"/last_blocks",
			http.StatusBadRequest,
			"400 Bad Request - Get last 1 blocks failed: gatewayGetLastBlocksError",
			httpBody{
				Num: "1",
			},
			1,
			&visor.ReadableBlocks{},
			errors.New("gatewayGetLastBlocksError"),
			&visor.ReadableBlocks{},
		},
		{
			"200",
			http.MethodGet,
			"/last_blocks",
			http.StatusOK,
			"",
			httpBody{
				Num: "1",
			},
			1,
			&visor.ReadableBlocks{},
			nil,
			&visor.ReadableBlocks{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			gateway := &FakeGateway{
				t: t,
			}

			gateway.On("GetLastBlocks", tc.num).Return(tc.gatewayGetLastBlocksResult, tc.gatewayGetLastBlocksError)
			v := url.Values{}
			var url = tc.url
			if tc.body.Num != "" {
				v.Add("num", tc.body.Num)
			}
			if len(v) > 0 {
				url += "?" + v.Encode()
			}

			req, err := http.NewRequest(tc.method, url, nil)
			require.NoError(t, err)
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(getLastBlocks(gateway))

			handler.ServeHTTP(rr, req)

			status := rr.Code
			require.Equal(t, tc.status, status, "case: %s, handler returned wrong status code: got `%v` want `%v`", tc.name, status, tc.status)

			if status != http.StatusOK {
				require.Equal(t, tc.err, strings.TrimSpace(rr.Body.String()), "case: %s, handler returned wrong error message: got `%v`| %s, want `%v`",
					tc.name, strings.TrimSpace(rr.Body.String()), status, tc.err)
			} else {
				var msg *visor.ReadableBlocks
				err := json.Unmarshal(rr.Body.Bytes(), &msg)
				require.NoError(t, err)
				require.Equal(t, tc.response, msg)
			}
		})
	}
}
