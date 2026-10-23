const player = document.getElementById("player");
const tracksEl = document.getElementById("tracks");
const playpause = document.getElementById("playpause");
const seek = document.getElementById("seek");
const timeEl = document.getElementById("time");
const mute = document.getElementById("mute");
const volume = document.getElementById("volume");
const audioBtn = document.getElementById("audioBtn");
const subBtn = document.getElementById("subBtn");
const menu = document.getElementById("menu");

let tracks = [];
let current = -1;
let seeking = false;
let audioOptions = [];
let subOptions = [];
let selectedAudio = null; // stream index string, null = default
let defaultAudioId = null;
let selectedSub = null; // track id, null = off

function fmt(sec) {
  if (!isFinite(sec)) return "0:00";
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

function mediaURL() {
  const base = `/media/${encodeURIComponent(tracks[current].id)}`;
  if (selectedAudio && selectedAudio !== defaultAudioId) {
    return `${base}?audio=${selectedAudio}`;
  }
  return base;
}

function applySubtitle() {
  [...player.querySelectorAll("track")].forEach((t) => t.remove());
  if (!selectedSub) return;
  const el = document.createElement("track");
  el.kind = "subtitles";
  el.src = `/subs/${encodeURIComponent(tracks[current].id)}?track=${encodeURIComponent(selectedSub)}`;
  el.default = true;
  player.appendChild(el);
  if (el.track) el.track.mode = "showing";
  el.addEventListener("load", () => (el.track.mode = "showing"));
}

function pickPreferred(options, storageKey, fallbackToDefault) {
  const pref = localStorage.getItem(storageKey);
  if (pref) {
    const match = options.find((o) => o.lang === pref);
    if (match) return match.id;
  }
  if (fallbackToDefault) {
    const def = options.find((o) => o.default);
    if (def) return def.id;
  }
  return null;
}

async function play(i) {
  if (i < 0 || i >= tracks.length) return;
  current = i;
  const id = tracks[i].id;

  const info = await fetch(`/api/info/${encodeURIComponent(id)}`).then((r) => r.json());
  audioOptions = info.audio || [];
  subOptions = info.subtitles || [];
  defaultAudioId = (audioOptions.find((a) => a.default) || {}).id || null;
  selectedAudio = pickPreferred(audioOptions, "prefAudioLang", true);
  selectedSub = pickPreferred(subOptions, "prefSubLang", false);

  player.src = mediaURL();
  applySubtitle();
  const resumeAt = Number(localStorage.getItem(`resume:${id}`) || 0);
  // metadata load hone se pehle seek mat karo (streamed mp4 ki duration estimate kharab hoti hai)
  player.addEventListener("loadedmetadata", () => (player.currentTime = resumeAt), { once: true });
  player.play();
  render();
}

function reloadKeepingPosition() {
  const t = player.currentTime;
  const wasPlaying = !player.paused;
  player.src = mediaURL();
  player.addEventListener(
    "loadedmetadata",
    () => {
      player.currentTime = t;
      if (wasPlaying) player.play();
    },
    { once: true }
  );
}

function render() {
  tracksEl.innerHTML = "";
  tracks.forEach((t, i) => {
    const div = document.createElement("div");
    div.className = "track" + (i === current ? " active" : "");
    div.textContent = t.name;
    div.onclick = () => play(i);
    tracksEl.appendChild(div);
  });
}

function showMenu(anchor, items, selectedId, onSelect) {
  menu.innerHTML = "";
  if (items.length === 0) {
    const empty = document.createElement("div");
    empty.className = "menu-item";
    empty.textContent = "No options";
    menu.appendChild(empty);
  }
  items.forEach((item) => {
    const row = document.createElement("div");
    row.className = "menu-item" + (item.id === selectedId ? " selected" : "");
    row.textContent = item.label;
    row.onclick = () => {
      onSelect(item.id);
      menu.classList.add("hidden");
    };
    menu.appendChild(row);
  });

  const rect = anchor.getBoundingClientRect();
  menu.style.visibility = "hidden";
  menu.classList.remove("hidden");
  requestAnimationFrame(() => {
    menu.style.left = rect.left + "px";
    menu.style.top = rect.top - menu.offsetHeight - 8 + "px";
    menu.style.visibility = "visible";
  });
}

audioBtn.onclick = (e) => {
  e.stopPropagation();
  showMenu(audioBtn, audioOptions, selectedAudio ?? defaultAudioId, (id) => {
    selectedAudio = id;
    const chosen = audioOptions.find((a) => a.id === id);
    localStorage.setItem("prefAudioLang", (chosen && chosen.lang) || "");
    reloadKeepingPosition();
  });
};

subBtn.onclick = (e) => {
  e.stopPropagation();
  const items = [{ id: "", label: "Off" }, ...subOptions];
  showMenu(subBtn, items, selectedSub || "", (id) => {
    selectedSub = id || null;
    const chosen = subOptions.find((s) => s.id === id);
    localStorage.setItem("prefSubLang", (chosen && chosen.lang) || "");
    applySubtitle();
  });
};

document.addEventListener("click", () => menu.classList.add("hidden"));

playpause.onclick = () => (player.paused ? player.play() : player.pause());
document.getElementById("prev").onclick = () => play(current - 1);
document.getElementById("next").onclick = () => play(current + 1);
document.getElementById("fullscreen").onclick = () => player.requestFullscreen();
mute.onclick = () => (player.muted = !player.muted);

volume.oninput = () => {
  player.volume = volume.value / 100;
  localStorage.setItem("volume", volume.value);
};

seek.addEventListener("input", () => (seeking = true));
seek.addEventListener("change", () => {
  if (player.duration) player.currentTime = (seek.value / 1000) * player.duration;
  seeking = false;
});

player.addEventListener("play", () => (playpause.textContent = "⏸"));
player.addEventListener("pause", () => (playpause.textContent = "▶"));
player.addEventListener("ended", () => play(current + 1));

player.addEventListener("timeupdate", () => {
  if (!seeking && player.duration) {
    seek.value = (player.currentTime / player.duration) * 1000;
  }
  timeEl.textContent = `${fmt(player.currentTime)} / ${fmt(player.duration)}`;
  if (current >= 0) {
    localStorage.setItem(`resume:${tracks[current].id}`, player.currentTime);
  }
});

const savedVolume = localStorage.getItem("volume");
if (savedVolume !== null) {
  volume.value = savedVolume;
  player.volume = savedVolume / 100;
}

function refreshLibrary() {
  return fetch("/api/library")
    .then((r) => r.json())
    .then((data) => {
      tracks = data || [];
      render();
    });
}

const uploadBtn = document.getElementById("uploadBtn");
const uploadInput = document.getElementById("uploadInput");

uploadBtn.onclick = () => uploadInput.click();

uploadInput.onchange = async () => {
  const files = [...uploadInput.files];
  if (files.length === 0) return;
  uploadBtn.disabled = true;
  uploadBtn.textContent = "…";
  for (const file of files) {
    const form = new FormData();
    form.append("file", file, file.name);
    try {
      const res = await fetch("/api/upload", { method: "POST", body: form });
      if (!res.ok) alert(`${file.name}: ${await res.text()}`);
    } catch (e) {
      alert(`${file.name}: ${e.message}`);
    }
  }
  uploadBtn.disabled = false;
  uploadBtn.textContent = "+";
  uploadInput.value = "";
  refreshLibrary();
};

refreshLibrary();

if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js");
}
