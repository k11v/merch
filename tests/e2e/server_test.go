package e2e

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/k11v/merch/api/merch"
)

func TestServer(t *testing.T) {
	t.Run("allows to buy an item", func(t *testing.T) {
		ctx := context.TODO()
		client := newTestClient(t)

		authResp, err := client.PostAPIAuthWithResponse(ctx, merch.PostAPIAuthJSONRequestBody{
			Username: "testuser",
			Password: "testpassword",
		})
		if err != nil {
			t.Fatalf("PostAPIAuthWithResponse: %v", err)
		}
		if authResp.JSON200 == nil {
			t.Fatalf("PostAPIAuthWithResponse: body is not JSON200")
		}
		token := *authResp.JSON200.Token

		buyResp, err := client.GetAPIBuyItemWithResponse(ctx, "t-shirt", authorization(token))
		if err != nil {
			t.Fatalf("GetAPIBuyItemWithResponse: %v", err)
		}
		if got, want := buyResp.StatusCode(), 200; got != want {
			t.Fatalf("GetAPIBuyItemWithResponse: got %d status code, want %d", got, want)
		}
		if len(buyResp.Body) != 0 {
			t.Fatalf("GetAPIBuyItemWithResponse: got non-empty body")
		}

		infoResp, err := client.GetAPIInfoWithResponse(ctx, authorization(token))
		if err != nil {
			t.Fatalf("GetAPIInfoWithResponse: %v", err)
		}
		if infoResp.JSON200 == nil {
			t.Fatalf("GetAPIInfoWithResponse: body is not JSON200")
		}
		inventory := *infoResp.JSON200.Inventory

		itemAmountFromName := make(map[string]int)
		for _, item := range inventory {
			itemAmountFromName[*item.Type] = *item.Quantity
		}
		if got, want := itemAmountFromName["t-shirt"], 1; got != want {
			t.Fatalf("got %d items, want %d", got, want)
		}
	})

	t.Run("allows to send coins", func(t *testing.T) {
		ctx := context.TODO()
		client := newTestClient(t)

		auth1Resp, err := client.PostAPIAuthWithResponse(ctx, merch.PostAPIAuthJSONRequestBody{
			Username: "testuser1",
			Password: "testpassword1",
		})
		if err != nil {
			t.Fatalf("PostAPIAuthWithResponse: %v", err)
		}
		if auth1Resp.JSON200 == nil {
			t.Fatalf("PostAPIAuthWithResponse: body is not JSON200")
		}
		if auth1Resp.JSON200.Token == nil {
			t.Fatalf("PostAPIAuthWithResponse: missing token body value")
		}
		token1 := *auth1Resp.JSON200.Token

		auth2Resp, err := client.PostAPIAuthWithResponse(ctx, merch.PostAPIAuthJSONRequestBody{
			Username: "testuser2",
			Password: "testpassword2",
		})
		if err != nil {
			t.Fatalf("PostAPIAuthWithResponse: %v", err)
		}
		if auth2Resp.JSON200 == nil {
			t.Fatalf("PostAPIAuthWithResponse: body is not JSON200")
		}
		if auth2Resp.JSON200.Token == nil {
			t.Fatalf("PostAPIAuthWithResponse: missing token body value")
		}
		token2 := *auth2Resp.JSON200.Token

		sendResp, err := client.PostAPISendCoinWithResponse(
			ctx,
			merch.PostAPISendCoinJSONRequestBody{
				ToUser: "testuser2",
				Amount: 15,
			},
			authorization(token1),
		)
		if err != nil {
			t.Fatalf("PostAPISendCoinWithResponse: %v", err)
		}
		if got, want := sendResp.StatusCode(), 200; got != want {
			t.Fatalf("PostAPISendCoinWithResponse: got %d status code, want %d", got, want)
		}
		if len(sendResp.Body) != 0 {
			t.Fatalf("PostAPISendCoinWithResponse: got non-empty body")
		}

		info1Resp, err := client.GetAPIInfoWithResponse(ctx, authorization(token1))
		if err != nil {
			t.Fatalf("GetAPIInfoWithResponse: %v", err)
		}
		if info1Resp.JSON200 == nil {
			t.Fatalf("GetAPIInfoWithResponse: body is not JSON200")
		}
		coins1 := *info1Resp.JSON200.Coins
		coinHistory1 := *info1Resp.JSON200.CoinHistory

		info2Resp, err := client.GetAPIInfoWithResponse(ctx, authorization(token2))
		if err != nil {
			t.Fatalf("GetAPIInfoWithResponse: %v", err)
		}
		if info2Resp.JSON200 == nil {
			t.Fatalf("GetAPIInfoWithResponse: body is not JSON200")
		}
		coins2 := *info2Resp.JSON200.Coins
		coinHistory2 := *info2Resp.JSON200.CoinHistory

		type sentCoinHistoryItem = struct {
			Amount *int    `json:"amount,omitempty"`
			ToUser *string `json:"toUser,omitempty"`
		}
		type receivedCoinHistoryItem = struct {
			Amount   *int    `json:"amount,omitempty"`
			FromUser *string `json:"fromUser,omitempty"`
		}
		var sentCoinHistory1 []sentCoinHistoryItem = *coinHistory1.Sent
		var receivedCoinHistory2 []receivedCoinHistoryItem = *coinHistory2.Received

		if got, want := coins1, 985; got != want {
			t.Fatalf("got %+v coins, want %+v", got, want)
		}
		if got, want := sentCoinHistory1, []sentCoinHistoryItem{{Amount: newInt(15), ToUser: newString("testuser2")}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v sent coin history, want %+v", got, want)
		}
		if got, want := coins2, 1015; got != want {
			t.Fatalf("got %+v coins, want %+v", got, want)
		}
		if got, want := receivedCoinHistory2, []receivedCoinHistoryItem{{Amount: newInt(15), FromUser: newString("testuser1")}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v received coin history, want %+v", got, want)
		}
	})
}

func newTestClient(tb testing.TB) *merch.ClientWithResponses {
	httpClient := new(http.Client)
	baseURL := "http://127.0.0.1:8080"
	client, err := merch.NewClientWithResponses(baseURL, merch.WithHTTPClient(httpClient))
	if err != nil {
		tb.Fatalf("got %v", err)
	}
	return client
}

func authorization(token string) merch.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func newInt(i int) *int {
	return &i
}

func newString(s string) *string {
	return &s
}
