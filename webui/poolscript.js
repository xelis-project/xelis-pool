

(() => {

	const addressIn = document.getElementById("wallet-address");
	const minerArea = document.getElementById("minerArea");

	addressIn.value = localStorage.getItem("addr");

	addressIn.addEventListener("change", () => {
		localStorage.setItem("addr", addressIn.value);
		refresh()
	})

	function refresh() {
		fetch(API_URL + "/stats").then(r => r.json()).then((d) => {
			document.getElementById("nethr").innerText = formatHashes(d.net_hr) + "H/s"
			document.getElementById("height").innerText = d.height
			document.getElementById("blocksfound").innerText = d.num_blocks_found
			document.getElementById("effort").innerText = (d.effort * 100).toFixed(1) + "%";

			// pool hr sparkline
			let chartData = [];
			for (e of d.chart.hashrate) {
				chartData.push({
					date: e.t * 1000,
					value: e.h,
				})
			}
			let poolHrText = document.getElementById("pool-hr-text")
			poolHrText.innerText = formatHashes(d.pool_hr) + "H/s"
			sparkline.sparkline(document.getElementById("pool-hr-chart"), chartData, {
				onmousemove(_, datapoint) {
					poolHrText.innerText = formatHashes(datapoint.value) + "H/s"
				},
				onmouseout() {
					poolHrText.innerText = formatHashes(d.pool_hr) + "H/s"
				}
			});

			// workers count sparkline
			if (d.chart.workers) {
				chartData = [];
				for (e of d.chart.workers) {
					chartData.push({
						date: 0,
						value: e,
					})
				}
				let poolWorkers = document.getElementById("workers-text")
				poolWorkers.innerText = d.connected_workers
				sparkline.sparkline(document.getElementById("workers-chart"), chartData, {
					onmousemove(ev, datapoint) {
						poolWorkers.innerText = datapoint.value
					},
					onmouseout() {
						poolWorkers.innerText = d.connected_workers
					}
				});
			}

			if (d.chart.addresses) {
				chartData = [];
				for (e of d.chart.addresses) {
					chartData.push({
						date: 0,
						value: e,
					})
				}
				let poolAddresses = document.getElementById("addresses-text")
				poolAddresses.innerText = d.connected_addresses
				sparkline.sparkline(document.getElementById("addresses-chart"), chartData, {
					onmousemove(ev, datapoint) {
						poolAddresses.innerText = datapoint.value
					},
					onmouseout() {
						poolAddresses.innerText = d.connected_addresses
					}
				});
			}

			document.querySelectorAll("svg").forEach((e)=>{
				e.setAttribute("viewBox", "0 0 200 100")	
				e.setAttribute("preserveAspectRatio", "none")
			})


		}).catch((e) => {
			console.error("error:" + e.toString())
			console.error("error: " + e.toString())
		})

		if (addressIn.value.length > 10) {
			fetch(API_URL + "/stats/" + addressIn.value).then(r => r.json()).then((d) => {
				console.log(d)

				if (d.error) {
					alert("could not find miner: " + d.error.message)
					
					return
				}

				document.getElementById("hashrate").innerText = formatHashes(d.hashrate || 0) + "H/s";


				document.getElementById("totaldue").innerText = (
					Math.round(
						((d.balance || 0))*1e5
					)/1e5
				) + " " + ticker;

				document.getElementById("unconfirmed").innerText = (
					Math.round((d.balance_pending || 0)*1e5)/1e5
				) + " " + ticker;

				document.getElementById("paid").innerText = (
					Math.round((d.paid || 0)*1e3)/1e3
				) + " " + ticker;
				
				let chartDom = document.getElementById("minerChart");
				if (d.hr_chart) {
					chartDom.style.display="block";
					var myChart = echarts.init(chartDom);

					const kol = {color: "#2039c9"}
						
					const option = {
						xAxis: {
							type: 'category',
							data: [],
						},
						yAxis: {
							type: 'value',
							axisLabel: {
								formatter: val => `${ formatHashes(val)}H/s`
							}
						},
						tooltip : {
							trigger: 'axis'
						},				
						series: [
							{
								name: "Your hashrate",
								data: [],
								type: 'line',
								showSymbol: false,
								lineStyle: kol,
								itemStyle: kol
							}
						]
					};
	
					for (v of d.hr_chart) {
						option.xAxis.data.push(new Date(v.t*1000).toLocaleTimeString())
						option.series[0].data.push(v.h)
					}
	
					myChart.setOption(option);
					window.addEventListener("resize", function () {
						myChart.resize();
					});
					setTimeout(myChart.resize, 1)	
				} else {
					chartDom.style.display="none";
				}


				const payouts = document.getElementById("payouts");
				payouts.innerHTML = "";

				for (e of d.withdrawals) {
					payouts.innerHTML += `<tr>`+
					`<td><a class="txhash" href="${EXPLORER_TX_URL + e.txid}" target="_blank">${e.txid}</a></td>`+
					`<td>${e.amount.toPrecision(6)} ${ticker}</td>`+
					`<td>${(new Date(e.time*1000)).toLocaleString()}</td>`+
					`</tr>`
				}


				minerArea.style.display = "block";
			}).catch((e) => {
				console.error(e.toString())
				alert("error: " + e.toString())
			})
		} else {
			minerArea.style.display = "none";
		}
	}
	refresh()

	setInterval(refresh, 2 * 60 * 1000)
})();