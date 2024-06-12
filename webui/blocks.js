const SHORT_TERM = 10;
(() => {
	fetch(API_URL + "/stats").then(r => r.json()).then((r) => {
		console.log(r)

		const blocksFound = document.getElementById("blocksFound")

		blocksFound.innerHTML = ""

		var effortSum = 0;
		var effortSumS = 0;
		var count = 0;
		var firstBlockTime = r.recent_blocks_found[r.recent_blocks_found.length-1].time

		for (e of r.recent_blocks_found) {
			if (count < SHORT_TERM) {
				effortSumS += e.effort
			}
			effortSum += e.effort;
			count++;
		}

		const longTermEffort = effortSum / count;

		for (e of r.recent_blocks_found) {
			const tr = document.createElement("tr")

			/*const tdHeight = document.createElement("td")
			tdHeight.innerText = e.height*/

			const tdHash = document.createElement("td")
			const hash = document.createElement("a")
			hash.href = EXPLORER_BLOCK_URL + e.hash
			hash.innerText = e.hash// .substring(0, 10)+"..."+e.hash.substring(e.hash.length - 10)
			hash.target = "_blank"
			hash.classList.add("txhash")
			tdHash.appendChild(hash)

			// tr.appendChild(tdHeight)
			tr.appendChild(tdHash)

			const tdEffort = document.createElement("td")
			tdEffort.innerText = (e.effort * 100 / longTermEffort).toFixed(1) + "%";
			tr.appendChild(tdEffort)

			const tdTime = document.createElement("td")
			tdTime.innerText = (new Date(e.time * 1000)).toLocaleString()
			tr.appendChild(tdTime)

			blocksFound.appendChild(tr)
		}



		document.getElementById("effortL").innerText = (effortSum / count * 100  / longTermEffort).toFixed(1)+"%"
		document.getElementById("effortS").innerText = (effortSumS / SHORT_TERM * 100  / longTermEffort).toFixed(1)+"%"

		const deltaT = (Date.now()/1000)-firstBlockTime

		const chunk = count/deltaT*15;

		document.getElementById("estChunk").innerText = (chunk*100).toFixed(2) +" %"
		document.getElementById("estActual").innerText = formatHashes(r.pool_hr / chunk)+"H/s"


	}).catch(err => {
		console.error(err)
	})

	setTimeout(location.reload, 3600 * 1e3)
})()
