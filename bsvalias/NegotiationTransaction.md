# Negotiation Transaction

## Terms

* BRFC (Bitcoin Request For Comments) is used to specify paymail features and endpoints. The BRFCID is used to uniquely identify paymail features.

## Negotiation Transaction BRFC

| Field    | Value                    |
|----------|--------------------------|
| Title    | Negotiation Transaction  |
| Author   | Curtis Ellis (Tokenized) |
| Version  | 1                        |
| BRFCID   | 27d8bd77c113             |

Negotiation Transaction is a dynamic endpoint that can be used to negotiate transfers of any sort. It enables conversations by tagging messages with a unique ID and enables delayed responses via a provided callback paymail handle. The initiator of the conversation is expected to post an incomplete transaction to the other party's paymail service.

### Data Structure

The body of a Negotiation Transaction HTTP request uses the following JSON structure.

```
{
	"id": "Unique Conversation Identifier"
	"handle": "Callback Paymail Handle",
	"fees":[
		{
			// The minimum number of satoshis per the number of bytes for the specified fee type.
			"feeType": "standard or data",
			"satoshis": 50,
			"bytes": 1000
		}
	],
	"expanded_tx": {
		"tx": "hex encoded raw bitcoin transaction",
		"ancestors": [
			{
				"tx": "hex encoded raw bitcoin transaction",
				"merkle_proofs": [
					{standard JSON merkle proof}
				],
				"miner_responses": [
					{standard JSON envelope containing relevant merchant-api responses}
				]
			}
		],
		"spent_outputs": [
			{
				"value": 1,
				"locking_script": "hex encoded locking script",
			}
		]
	}
}
```

## Merkle Proofs BRFC

| Field    | Value                    |
|----------|--------------------------|
| Title    | Merkle Proofs  |
| Author   | Curtis Ellis (Tokenized) |
| Version  | 1                        |
| BRFCID   | b38a1b09c3ce             |

Merkle Proofs is an endpoint that allows posting of merkle proofs to paymail following an exchange of a transaction that one of the parties broadcast to the Bitcoin miners. The party that broadcast the transaction is expected to deliver all relevant merkle proofs to the other party. The transaction id (double SHA256) contained in the merkle proofs is considered enough identifying information for the paymail service to know what to do with it.

### Data Structure

The body of a Negotiation Transaction HTTP request uses the following JSON structure.

```
[
	{standard JSON merkle proof}
]
```

## Workflows

### Simple Unsolicited Tokenized Payment

Sender wants to send some Tokenized tokens to a paymail recipient.

1. The sender posts negotiation transaction to recipient paymail that includes a unique ID, the sender's paymail handle, and a transaction containing a Tokenized (T1) Transfer action with senders and possibly change receivers. The sum of the quantity of the senders minus the sum of the quantity of the receivers is the quantity that is to be paid to the recipient. The transaction must contain at a minimum the outputs being spent by the inputs of the transaction and possibly full ancestor transactions for the inputs.
2. The recipient paymail service verifies the token is supported and adds transfer receivers and responds to the HTTP request with the updated negotiation transaction containing the same unique ID and the recipient's paymail handle.
3. The sender verifies that the negotiation transaction has all necessary receivers (Tokenized Transfers must have matching sender and receiver quantity sums) and updates the contract and tx fees, adds any inputs for transaction funding, signs all inputs, and posts the completed negotiation transaction back to recipient's paymail with the same unique ID and the sender's paymail handle.
4. The recipient paymail service verifies the transfer tx is not malformed and broadcasts to the miners. If the broadcast fails the recipient paymail service responds to the HTTP request with a 400 (Bad Request) and a text body containing a description of the issue as received from the miners. If the broadcast succeeds then the recipient paymail service responds to the HTTP request with a 200 (OK).
5. The recipient later receives the transaction containing the Tokenized (T2) Settlement action from the smart contract agent and posts it in a negotiation transaction to the sender's paymail handle.
6. The recipient later receives merkle proofs for the transactions containing the Tokenized (T1) Transfer and Tokenized (T2) Settlement actions and posts to sender's paymail merkle proofs endpoint.
