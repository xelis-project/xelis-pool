const API_URL = "https://api.xelpool.com"; // Example, change it to your pool's API URL
const EXPLORER_TX_URL = "https://explorer.xelis.io/txs/"
const EXPLORER_BLOCK_URL = "https://explorer.xelis.io/blocks/"

function formatHashes(n) {
	if (n >= 1e12) {
		return (n / 1e12).toFixed(2) + " T"
	} else if (n >= 1e9) {
		return (n / 1e9).toFixed(2) + " G"
	} if (n >= 1e6) {
		return (n / 1e6).toFixed(2) + " M"
	} else if (n >= 1e3) {
		return (n / 1e3).toFixed(2) + " k"
	}
	return Math.round(n).toString() + " "
}
