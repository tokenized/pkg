# Negotiation Transaction

## Terms

* BRFC (Bitcoin Request For Comments) is used to specify paymail features and endpoints. The BRFCID is used to uniquely identify paymail features.

## Warnings

There is the risk that during exchange and modification of the negotiation transaction that the other party could try to get you to sign something that you don't want to.

### Examples

* If the other party knows your UTXOs, they could try to add your UTXOs as inputs and if you don't check you could sign it simply because your wallet recognizes it as yours and it is in the transaction. Depending on how your wallet does signing.
* The other party could add some higher level protocol data that your wallet doesn't recognize to the transaction like agreeing to a legal contract or signing for a token protocol not supported by your software.

### Safety Guidelines

* Keep track of which inputs you add to the tx and make sure your wallet only signs those.
* If there are any unrecognized output locking script formats, especially `OP_FALSE OP_RETURN`, then abort the negotiation with a 406 (Not Acceptable).

## Negotiation Transaction BRFC

| Field    | Value                    |
|----------|--------------------------|
| Title    | Negotiation Transaction  |
| Author   | Curtis Ellis (Tokenized) |
| Version  | 1                        |
| BRFCID   | 27d8bd77c113             |

Negotiation Transaction is a dynamic endpoint that can be used to negotiate transfers of any sort.

Features:

# Conversations

By tagging messages with a unique ID several messages can be exchanged and linked to the same conversation "thread".

# Delayed Responses

By including a paymail handle and/or peer channels the paymail service doesn't have to provide the response immediately. The paymail service can provide the message to the user and let the user respond.

# Callbacks

Tokenized (T2) settlements, merkle proofs, and other data needed after a transaction is complete can be provided via the included paymail handle and peer channels.

### HTTP Response Codes

* 200 (OK) If there is a body then it contains the next step of the negotiation transaction. If there is not body the negotiation completed successfully.
* 202 (Accepted) The negotiation transaction requires interaction with the user so a response will be posted to the paymail handle provided.
* 400 (Bad Request) The request is invalid. The body will contain a text description of the issue.
* 406 (Not Acceptable) The request contained protocols or scenarios not supported by this paymail service implementation. The body will contain a text description of the issue.

### Data Structure

The body of a Negotiation Transaction HTTP request uses the following JSON structure.

```
{
	"id": "Unique Conversation Identifier",
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
	},
	"handle": "Callback Paymail Handle",
	"peer_channels": [
		{
			"url": "peer channel post URL including channel id",
			"write_token": "HTTP auth header that allows posting messages"
		}
	]
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

### HTTP Response Codes

* 200 (OK) The merkle proofs were received successfully.
* 400 (Bad Request) One of the merkle proofs were invalid. The body will contain a text description of the issue.

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

1. The sender posts a negotiation transaction to recipient paymail that includes a unique ID, the sender's paymail handle, and a transaction containing a Tokenized (T1) Transfer action with senders and possibly change receivers. The sum of the quantity of the senders minus the sum of the quantity of the receivers is the quantity that is to be paid to the recipient. The transaction must contain at a minimum the outputs being spent by the inputs of the transaction and possibly full ancestor transactions for the inputs.
2. The recipient paymail service verifies the token is supported and adds transfer receivers and responds to the HTTP request with 200 (OK) and the updated negotiation transaction containing the same unique ID and the recipient's paymail handle.
3. The sender verifies that the negotiation transaction has all necessary receivers (Tokenized Transfers must have matching sender and receiver quantity sums) and updates the contract and tx fees, adds any inputs for transaction funding, signs all inputs, and posts the completed negotiation transaction back to recipient's paymail with the same unique ID and the sender's paymail handle.
4. The recipient paymail service verifies the transfer tx is not malformed and broadcasts to the miners. If the broadcast fails the recipient paymail service responds to the HTTP request with a 400 (Bad Request) and a text body containing a description of the issue as received from the miners. If the broadcast succeeds then the recipient paymail service responds to the HTTP request with a 200 (OK).
5. The recipient later receives the transaction containing the Tokenized (T2) Settlement action from the smart contract agent and posts it in a negotiation transaction to the sender's paymail handle.
6. The recipient later receives merkle proofs for the transactions containing the Tokenized (T1) Transfer and Tokenized (T2) Settlement actions and posts to sender's paymail merkle proofs endpoint.
