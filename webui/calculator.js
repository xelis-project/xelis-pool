const blockTime = 15;

(() => {
	let blockReward = 0;
	let netHr = 0;



	function refreshCfg() {
		const yHr = parseFloat(document.getElementById("yourHashrate").value) || 0
		document.getElementById("estimateProfit").innerText = (yHr/netHr*blockReward*(3600*24/blockTime)).toFixed(3);
	}
	setInterval(refreshCfg, 100)


	fetch(API_URL+"/stats").then(r => r.json()).then((r) => {
		blockReward = r.reward;
		netHr = r.net_hr/1000;

		console.log("block reward",blockReward,"net hashrate",netHr)
		refreshCfg()

		console.log(r)
	}).catch(err=>{
		console.error(err)
	})
})()

setTimeout(location.reload,3600*1e3)
