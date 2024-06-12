
addEventListener("hashchange", () => {
	loadSpa(location.hash.substring(1))

});


function loadSpa(url) {
	if (url === "home") return;

	const spaMain = document.getElementById("spa-main")

	fetch("/"+url+".html").then((r)=>r.text()).then((r)=>{
		spaMain.innerHTML = r;
	})
}
loadSpa(location.hash.substring(1))
