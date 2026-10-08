const grid = document.getElementById("game-grid");
const searchInput = document.getElementById("search");
const genreSelect = document.getElementById("genre-filter");
const sortSelect = document.getElementById("sort");
const countText = document.getElementById("count");

const PLACEHOLDER = "data:image/svg+xml," + encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400">' +
  '<rect width="100%" height="100%" fill="#2a2f3a"/>' +
  '<text x="50%" y="50%" fill="#9aa1ad" font-family="sans-serif" font-size="20" ' +
  'text-anchor="middle" dominant-baseline="middle">No Image</text></svg>'
);

function getGenres(list) {
  return [...new Set(list.map(game => game.genre))].sort();
}

function populateGenres() {
  getGenres(games).forEach(genre => {
    const option = document.createElement("option");
    option.value = genre;
    option.textContent = genre;
    genreSelect.appendChild(option);
  });
}

function filterGames(list, search, genre) {
  const term = search.trim().toLowerCase();
  return list.filter(game =>
    game.title.toLowerCase().includes(term) &&
    (genre === "all" || game.genre === genre)
  );
}

function sortGames(list, sortBy) {
  const sorted = [...list];
  switch (sortBy) {
    case "rating-asc": return sorted.sort((a, b) => a.rating - b.rating);
    case "title-asc":  return sorted.sort((a, b) => a.title.localeCompare(b.title));
    case "title-desc": return sorted.sort((a, b) => b.title.localeCompare(a.title));
    default:           return sorted.sort((a, b) => b.rating - a.rating);
  }
}

function createCard(game) {
  const card = document.createElement("article");
  card.className = "card";

  const img = document.createElement("img");
  img.src = game.image;
  img.alt = game.title;
  img.loading = "lazy";
  img.onerror = () => { img.onerror = null; img.src = PLACEHOLDER; };

  const body = document.createElement("div");
  body.className = "card-body";

  const title = document.createElement("h2");
  title.className = "card-title";
  title.textContent = game.title;

  const meta = document.createElement("div");
  meta.className = "card-meta";
  meta.append(createTag(game.genre), createTag(game.platform));

  const rating = document.createElement("div");
  rating.className = "rating";
  rating.textContent = `★ ${game.rating.toFixed(1)}`;

  body.append(title, meta, rating);
  card.append(img, body);
  return card;
}

function createTag(text) {
  const tag = document.createElement("span");
  tag.className = "tag";
  tag.textContent = text;
  return tag;
}

function renderGames(list) {
  grid.innerHTML = "";
  countText.textContent = `${list.length} game${list.length === 1 ? "" : "s"}`;

  if (list.length === 0) {
    grid.innerHTML = '<p class="empty">No games found.</p>';
    return;
  }

  list.forEach(game => grid.appendChild(createCard(game)));
}

function update() {
  const filtered = filterGames(games, searchInput.value, genreSelect.value);
  renderGames(sortGames(filtered, sortSelect.value));
}

searchInput.addEventListener("input", update);
genreSelect.addEventListener("change", update);
sortSelect.addEventListener("change", update);

populateGenres();
update();
