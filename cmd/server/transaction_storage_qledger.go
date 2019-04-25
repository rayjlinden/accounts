// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/moov-io/gl"
	mledge "github.com/moov-io/qledger-sdk-go"

	"github.com/RealImage/QLedger/models"
)

func setupQLedgerTransactionStorage(endpoint, apiToken string) (*qledgerTransactionRepository, error) {
	if endpoint == "" || apiToken == "" {
		return nil, fmt.Errorf("qledger: empty endpoint=%q and/or apiToken", endpoint)
	}
	return &qledgerTransactionRepository{
		api: mledge.NewLedger(endpoint, apiToken),
	}, nil
}

type qledgerTransactionRepository struct {
	api *mledge.Ledger
}

func (r *qledgerTransactionRepository) Ping() error {
	return r.api.Ping()
}

// joinAccountIds returns a comma separated list of accountIDs from an array of transactionLines.
// We do this to query transactions that have been posted against an account.
func joinAccountIds(lines []transactionLine) string {
	out := ""
	for i := range lines[:1] {
		out += lines[i].AccountId + ","
	}
	return strings.TrimSuffix(out, ",")
}

func (r *qledgerTransactionRepository) createTransaction(account *gl.Account, tx transaction) error {
	var lines []*mledge.TransactionLine
	data := make(map[string]interface{})
	data["accountIds"] = joinAccountIds(tx.Lines)

	for i := range tx.Lines {
		lines = append(lines, &mledge.TransactionLine{
			AccountID: tx.Lines[i].AccountId,
			Delta:     tx.Lines[i].Amount,
		})
		// TODO(adam): https://github.com/RealImage/QLedger/issues/40
		// data[fmt.Sprintf("%s_purpose", tx.Lines[i].AccountId)] = tx.Lines[i].Purpose
	}

	return r.api.CreateTransaction(&mledge.Transaction{
		ID:        tx.ID,
		Data:      data,
		Timestamp: tx.Timestamp.Format(models.LedgerTimestampLayout),
		Lines:     lines,
	})
}

func (r *qledgerTransactionRepository) getAccountTransactions(accountId string) ([]transaction, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"must": map[string]interface{}{
				"ranges": []map[string]interface{}{
					{
						"accountIds": map[string]interface{}{
							"like": accountId,
						},
					},
				},
			},
		},
	}
	xfers, err := r.api.SearchTransactions(query)
	if err != nil {
		return nil, fmt.Errorf("qledger: getAccountTransactions: %v", err)
	}

	var transactions []transaction
	for i := range xfers {
		var lines []transactionLine
		for j := range xfers[i].Lines {
			// TODO(adam): https://github.com/RealImage/QLedger/issues/40
			// p, _ := xfers[i].Data[fmt.Sprintf("%s_purpose", xfers[i].Lines[j].AccountID)].(string)
			p := "achcredit"
			if xfers[i].Lines[j].Delta < 0 {
				p = "achdebit" // TODO(adam): mocked for tests, see commented '%s_purpose' above
			}
			lines = append(lines, transactionLine{
				AccountId: xfers[i].Lines[j].AccountID,
				Amount:    xfers[i].Lines[j].Delta,
				Purpose:   TransactionPurpose(p),
			})
		}
		t, _ := time.Parse(models.LedgerTimestampLayout, xfers[i].Timestamp)
		transactions = append(transactions, transaction{
			ID:        xfers[i].ID,
			Timestamp: t,
			Lines:     lines,
		})
	}
	return transactions, nil
}
