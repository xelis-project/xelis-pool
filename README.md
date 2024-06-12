# XELIS-POOL
Open-source, high performance XELIS mining pool.
Originally written from scratch in Go by [XelPool](https://XelPool.com/), it's now Free Software.

## Example configuration

```jsonc
{
	"LogLevel": 0,
	"MasterPass": "enter a secure password here",
	"Atomic": 8, // always 8 for XELIS
	"PoolAddress": "your pool address",
	"FeeAddress": "your fee address",
	"BlockTime": 15, // block time in seconds
	"AddressPrefix": "xel",
	"Slave": {
		"MasterAddress": "YOUR_MASTER_IPV4:3221",
		"InitialDifficulty": 25000000,
		"MinDifficulty": 100000,
		"ShareTarget": 30,
		"TrustScore": 50, // once a miner has sent 50 valid shares, mark it as trusted
		"TrustedCheckChance": 75, // only 75% of trusted shares are checked
		"XatumPort": 5212,
		"GetworkPort": 2086,
		"StratumPort": 9351
	},
	"Master": {
		"WalletRpc": "127.0.0.1:4111",
		"DaemonRpc": "127.0.0.1:8080",

		"WalletRpcUser": "user",
		"WalletRpcPass": "your wallet rpc password here",

		"MinConfs": 4,

		"Port": 3221,
		"ApiPort": 4006,
		"FeePercent": 1,

		"MinWithdrawal": 0.1,
		"WithdrawalFee": 0.0005
	}
}
```

## Web UI
An example Web UI can be found in the webui folder.

## License
XELIS-POOL is licensed under AGPL v3.

Permissions of this copyleft license are conditioned on making available complete source code of licensed works and modifications, which include larger works using a licensed work, under the same license. Copyright and license notices must be preserved. Contributors provide an express grant of patent rights. When a modified version is used to provide a service over a network, **the complete source code of the modified version must be made available**. 

Read more in the License file.
