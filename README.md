
# Ethereum Validator API

## Overview
This project implements an API for retrieving Ethereum validator information, specifically designed to:

- Calculate **Block Rewards** earned by validators for specific slots.
- Retrieve **Sync Committee Duties** for validators at given slots.

The solution leverages a **hexagonal architecture**, allowing loose coupling between business logic and external infrastructure, making it highly maintainable and testable.

## Project Structure

```
.
├── cmd/api              # Application entry point (main.go)
├── internal
│   ├── adapter          # External services (Consensus & Execution clients)
│   ├── domain           # Core domain models
│   ├── errors           # Application-specific error definitions
│   ├── handler          # HTTP handlers
│   ├── port             # Interfaces to abstract external adapters
│   ├── retry            # Retry utilities for transient errors
│   └── usecase          # Business logic for domain operations
├── pkg
│   ├── config           # Configuration loader
│   ├── http             # HTTP router setup
│   └── logger           # Logging setup (zap logger)
├── integration          # Integration tests
├── Makefile             # Commands to run/build/test the application
└── README.md            # This document
```

## Technology Stack

- **Go 1.24.5**
- **chi router**
- **zap logger**
- **QuickNode Ethereum endpoints**
- **golang-lru** for caching (LRU cache)

## Key Functionalities

### Block Reward Calculation

Given a slot, the API calculates rewards comparing balances before and after block production:

```
rewardWei = balanceAfterSlot - balanceBeforeSlot
rewardGwei = rewardWei / 1e9
```

### Alternative Approach to Calculate Block Reward
Initially, we explored an alternative method to calculate the validator's block reward using transaction receipts:

#### Method Description

The alternative consisted of fetching all the transactions included in a specific block and summing up the gas used multiplied by the effective gas price for each transaction:
```
Block Reward = Σ (effectiveGasPrice × gasUsed) for each transaction in block
```

#### Steps Involved
- Obtain the block details by its number (eth_getBlockByNumber).
- Retrieve each transaction's receipt included in the block (eth_getTransactionReceipt).
- Calculate the total gas used multiplied by the effective gas price per transaction.
- Sum these values to obtain the block reward.

#### Reasons for Not Including This Method
- Performance and Complexity: Requires multiple requests to the Ethereum node, significantly degrading performance.
- Complexity in Error Handling: Higher likelihood of intermittent errors, complicating error handling.
- API Credit Cost: Increased operational cost due to numerous requests

#### Chosen Method (Balance Difference)
We opted for the simpler and more efficient approach:

- Fetch the miner's balance immediately before and after the block.

- Calculate the reward as the difference between these two balances.

This simpler method drastically reduces complexity, API calls, and execution time, making it more reliable and performant.

Rewards classified as:

- **vanilla**: Without MEV relay.
- **mev**: Via MEV relay.

### Sync Duties Calculation

Retrieves validators with sync committee duties via:

```
GET /eth/v1/beacon/states/{slot}/sync_committees
```


## Cache Strategy (LRU Cache)

Implemented due to initial slow responses (6-7s) in Syn Duties Node Responses:

- Quick retrieval (ms after initial fetch).
- Entries expire (TTL configuration).
- Future improvement: Use Redis for persistence.

## ⚙️ How to Run

### Makefile:

```sh
make run
```

### Dcoker:

```sh
make build
```

```sh
make up
```

### Manually:

```sh
go run cmd/api/main.go
```

### Tests:

```sh
make test
make coverage
```

## API Usage

### Block Reward:

```sh
curl -i localhost:8080/blockreward/{slot_number}
```

```sh
curl -i 0.0.0.0:8080/blockreward/{slot_number}
```
Example response:
```
{"status":"mev","reward_gwei":3187542500}
{"status":"vanilla","reward_gwei":219817237}
```

### Sync Duties:

```sh
curl -i localhost:8080/syncduties/{slot_number}
```

```sh
curl -i localhost:0.0.0.0/syncduties/{slot_number}
```

Example response:

```
{"validators":["0xa63e0f5cc97436716d3f06d5a203d1599ed0c219dda21005eddb8d24c38fcb139aef505307e91f4e13798907c44a0b47","0xaae03d272c20faddc8b3d51b63880a9fb4abb48939d5963502b63574abb1943335be9c81c7d0a73f61807f40f0bdcff0"]}
```


## Hexagonal Architecture

Chosen for clear separation, testability, and flexibility.

## Retry Policy

To improve resilience and reliability, a retry mechanism has been implemented when calling external Ethereum clients (both execution and consensus layers).

### Why Use Retries?
Transient Errors: Ethereum nodes can occasionally return temporary errors or timeouts. Retries help overcome these without failing the whole request.

Improved User Experience: Users receive successful responses even when the first attempt to the node fails.

Controlled Backoff: Retries use configurable backoff strategies to avoid overwhelming the node and to increase chances of recovery.

### Retry Configuration
Defined per operation (e.g., BlockReward, SyncDuties), with parameters:

- Timeout

- MaxRetries

- Backoff duration (e.g., 100ms)

These settings can be adjusted in the config.json file for fine-grained control.

## Testing Strategy
This project employs a testing pyramid approach:

- Integration Tests (in integration/): End-to-end coverage using httptest.Server against both endpoints. These verify the full API behavior against mocked QuickNode responses and are run with make test.

- Unit Tests: Core business logic in usecase/ is covered by unit tests. Given the complexity and external dependencies of the consensus/execution adapters, their behavior is captured in integration tests, and adapter-level unit tests are omitted to avoid redundant mocking.

To run all tests:
```
make test
```


## Future Work & Improvements

### High Priority:

- Expand tests coverage (>70%).
- Persistent Redis cache.
- Revisit WebSocket proactive caching. Subscribe to newHeads and for each event precalculate the syncDuties to cache and serve them quickly. This idea was implemented but hasn't made it into the final version of this code.
- Dynamic MEV Relay Discovery: Integrate with an external MEV relay registry or Builder API (EIP-4844) to automatically fetch and update relay addresses, replacing the static configuration list for a more professional and maintainable approach.

### Medium Priority:

- API rate limiting.
- Metrics & monitoring (Prometheus).


## Error Handling

- **400**: Invalid request.
- **404**: Slot/state not found.
- **500**: Internal error.
- **504**: Gateway timeout.

## Configuration

```json
{
  "SERVER_ADDRESS": ":8080",
  "ETH_RPC_HTTP": "https://your_quicknode_url",
  "ETH_RPC_WS":   "wss://your_quicknode_ws_url",
  "MEV_RELAYS": ["relay1", "relay2"],
  "CACHE_SYNC_MAX_ENTRIES": 1024,
  "CACHE_SYNC_TTL": "60m",

  "BR_TIMEOUT": "5s",
  "BR_MAX_RETRIES": 3,
  "BR_BACKOFF": "100ms",

  "SD_TIMEOUT": "10s",
  "SD_MAX_RETRIES": 3,
  "SD_BACKOFF": "100ms"
}
```

## Resources

[QuickNode Doc Ethereum](https://www.quicknode.com/docs/ethereum)

[Gwei](https://www.risein.com/blog/what-is-gwei)

[Analyzing MEV](https://medium.com/@toni_w/practical-guide-into-analyzing-mev-in-the-proof-of-stake-era-e2b024509918)



