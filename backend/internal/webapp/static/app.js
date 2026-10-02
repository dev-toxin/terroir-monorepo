// Terroir PWA — vanilla JS, no build step. All user/API data is inserted via textContent.
"use strict";

(function () {
  const I18N = {
    ru: {
      logout: "Выйти", loginTitle: "Вход", loginHint: "Введите токен. Демо: dev-anton-token или dev-elena-token",
      loginBtn: "Войти", search: "Поиск", readyChip: "Пора пить", chatPh: "Что открыть к стейку?", send: "Отпр.",
      tabCellar: "Погреб", tabRecipes: "Рецепты", tabTaste: "Вкус", tabChat: "Чат",
      bottles: "бутылок", positions: "позиций", openSoon: "Открыть скоро", drunk30: "выпито за 30 дней",
      drink: "Выпил", empty: "Пусто", loading: "Загрузка…", rating: "Оценка", note: "Заметка", recipeOpt: "С каким блюдом (необязательно)",
      none: "— нет —", save: "Сохранить", cooked: "Что приготовить", whatOpen: "Что открыть из погреба", score: "балл",
      location: "Место", window: "Окно питья", price: "Цена", grapes: "Сорта", region: "Регион", notes: "Заметки",
      surveyDone: "Опрос пройден — спасибо!", surveyNext: "Ответить и дальше", answered: "Отвечено",
      profile: "Профиль вкуса", body: "Тело", acidity: "Кислотность", tannin: "Танины", sweetness: "Сладость",
      darkFruit: "Тёмные фрукты", spice: "Острота", liked: "Любимые сорта", rejected: "Не любит",
      noAnswers: "Выберите хотя бы один ответ", drunkOk: "Записано. Осталось: ", hello: "Спросите: «что открыть к болоньезе?», «что пора выпить?», «что купить снова?»",
      unauthorized: "Неверный токен", network: "Нет связи с сервером", nv: "Н/В", portion: "мин",
    },
    en: {
      logout: "Log out", loginTitle: "Sign in", loginHint: "Enter your token. Demo: dev-anton-token or dev-elena-token",
      loginBtn: "Sign in", search: "Search", readyChip: "Ready to drink", chatPh: "What to open with steak?", send: "Send",
      tabCellar: "Cellar", tabRecipes: "Recipes", tabTaste: "Taste", tabChat: "Chat",
      bottles: "bottles", positions: "items", openSoon: "Open soon", drunk30: "drunk in 30 days",
      drink: "Drank", empty: "Empty", loading: "Loading…", rating: "Rating", note: "Note", recipeOpt: "With which dish (optional)",
      none: "— none —", save: "Save", cooked: "What to cook", whatOpen: "What to open from the cellar", score: "score",
      location: "Location", window: "Drinking window", price: "Price", grapes: "Grapes", region: "Region", notes: "Notes",
      surveyDone: "Survey complete — thank you!", surveyNext: "Answer and continue", answered: "Answered",
      profile: "Taste profile", body: "Body", acidity: "Acidity", tannin: "Tannin", sweetness: "Sweetness",
      darkFruit: "Dark fruit", spice: "Spice", liked: "Liked grapes", rejected: "Rejected",
      noAnswers: "Pick at least one answer", drunkOk: "Saved. Left: ", hello: "Ask: \"what to open with bolognese?\", \"what should I drink now?\", \"what to buy again?\"",
      unauthorized: "Invalid token", network: "Cannot reach the server", nv: "NV", portion: "min",
    },
  };

  const state = {
    token: localStorage.getItem("terroir.token") || "",
    lang: localStorage.getItem("terroir.lang") || "ru",
    ready: false,
    q: "",
    tab: "cellar",
    conversationId: "",
    recipes: null,
  };

  const $ = (id) => document.getElementById(id);
  const t = (k) => (I18N[state.lang] && I18N[state.lang][k]) || I18N.ru[k] || k;

  // el("li", {class: "x", onclick: fn}, child1, "text", ...) — strings become text nodes (never HTML).
  function el(tag, attrs, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (v === undefined || v === null || v === false) continue;
      if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
      else if (k === "class") n.className = v;
      else n.setAttribute(k, v === true ? "" : String(v));
    }
    for (const c of children.flat()) {
      if (c === null || c === undefined || c === false) continue;
      n.appendChild(typeof c === "string" || typeof c === "number" ? document.createTextNode(String(c)) : c);
    }
    return n;
  }
  const clear = (n) => { while (n.firstChild) n.removeChild(n.firstChild); return n; };

  // ---------- API ----------
  class ApiError extends Error {
    constructor(status, message) { super(message); this.status = status; }
  }

  async function api(method, path, body) {
    const sep = path.includes("?") ? "&" : "?";
    const opts = { method, headers: { Authorization: "Bearer " + state.token } };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    let res;
    try {
      res = await fetch(path + sep + "lang=" + state.lang, opts);
    } catch (_) {
      throw new ApiError(0, t("network"));
    }
    let data = null;
    const text = await res.text();
    if (text) { try { data = JSON.parse(text); } catch (_) { data = null; } }
    if (!res.ok) {
      const msg = data && data.error && data.error.message ? data.error.message : "HTTP " + res.status;
      throw new ApiError(res.status, msg);
    }
    return data;
  }

  function showError(err) {
    const box = $("error");
    if (!err) { box.classList.add("hidden"); return; }
    if (err.status === 401) { logout(); box.textContent = t("unauthorized"); }
    else box.textContent = err.message || String(err);
    box.classList.remove("hidden");
    clearTimeout(showError.timer);
    showError.timer = setTimeout(() => box.classList.add("hidden"), 6000);
  }

  async function guarded(fn) {
    try { showError(null); await fn(); } catch (e) { showError(e); }
  }

  // ---------- i18n ----------
  function applyI18n() {
    document.documentElement.lang = state.lang;
    document.querySelectorAll("[data-i18n]").forEach((n) => { n.textContent = t(n.dataset.i18n); });
    document.querySelectorAll("[data-i18n-ph]").forEach((n) => { n.placeholder = t(n.dataset.i18nPh); });
    $("lang-btn").textContent = state.lang === "ru" ? "EN" : "RU";
  }

  // ---------- helpers ----------
  function wineTitle(w) {
    const v = w.vintage ? String(w.vintage) : t("nv");
    return w.name + (w.producer ? ", " + w.producer : "") + " " + v;
  }
  function windowBadge(v) {
    return el("span", { class: "badge w-" + (v.window || "unknown") }, v.window_label || v.window || "");
  }
  function money(minor, cur) {
    if (minor === null || minor === undefined) return "";
    return (minor / 100).toLocaleString(state.lang) + " " + (cur || "");
  }
  function windowText(it) {
    if (!it.drink_from && !it.drink_until) return "—";
    return (it.drink_from || "…") + "–" + (it.drink_until || "…");
  }
  function reasonsList(reasons) {
    return el("ul", { class: "reasons" }, (reasons || []).map((r) => el("li", {}, r)));
  }

  // ---------- modal ----------
  function openModal(...children) {
    const c = clear($("modal-content"));
    children.forEach((ch) => c.appendChild(ch));
    $("modal").classList.remove("hidden");
  }
  function closeModal() { $("modal").classList.add("hidden"); clear($("modal-content")); }

  // ---------- cellar ----------
  async function loadSummary() {
    const s = await api("GET", "/v1/cellar/summary?top=3");
    const box = clear($("summary"));
    box.append(
      el("span", { class: "big" }, s.total_bottles), el("span", {}, t("bottles")),
      el("span", { class: "muted" }, s.total_items + " " + t("positions") + " · " + s.drunk_last_30_days + " " + t("drunk30")),
    );
    if (s.open_soon && s.open_soon.length) {
      box.append(el("div", { class: "soon" }, t("openSoon") + ": ",
        s.open_soon.map((v, i) => el("span", {}, (i ? " · " : "") + v.wine.name + " (" + v.window_label + ")"))));
    }
  }

  async function loadCellar() {
    const list = clear($("cellar-list"));
    list.appendChild(el("li", { class: "muted" }, t("loading")));
    const params = new URLSearchParams();
    if (state.ready) params.set("ready_to_drink", "true");
    if (state.q) params.set("q", state.q);
    const qs = params.toString();
    const [data] = await Promise.all([api("GET", "/v1/cellar/items" + (qs ? "?" + qs : "")), loadSummary()]);
    clear(list);
    if (!data.items.length) { list.appendChild(el("li", { class: "muted" }, t("empty"))); return; }
    for (const v of data.items) {
      list.appendChild(el("li", {},
        el("span", { class: "dot c-" + v.wine.color }),
        el("div", { class: "info", onclick: () => guarded(() => openBottle(v.item.id)) },
          el("div", { class: "title" }, wineTitle(v.wine)),
          el("div", { class: "sub" }, [v.wine.region, v.item.storage_location].filter(Boolean).join(" · ")),
          windowBadge(v)),
        el("span", { class: "qty" }, "×" + v.item.quantity),
        el("button", { class: "primary", type: "button", disabled: v.item.quantity <= 0, onclick: () => guarded(() => openDrink(v)) }, t("drink")),
      ));
    }
  }

  async function openBottle(id) {
    const [v, recs] = await Promise.all([api("GET", "/v1/cellar/items/" + encodeURIComponent(id)), api("GET", "/v1/cellar/items/" + encodeURIComponent(id) + "/recipes?limit=5")]);
    const w = v.wine, it = v.item;
    const kv = el("dl", { class: "kv" });
    const row = (k, val) => { if (val) kv.append(el("dt", {}, t(k)), el("dd", {}, String(val))); };
    row("grapes", (w.grapes || []).join(", "));
    row("region", [w.country, w.region].filter(Boolean).join(", "));
    row("location", it.storage_location);
    row("window", windowText(it) + " · " + v.window_label);
    row("price", money(it.purchase_price_minor, it.currency));
    row("notes", [w.style_notes, it.notes].filter(Boolean).join(" · "));
    const recList = el("ul", { class: "list" }, recs.items.map((s) => el("li", {},
      el("div", { class: "info" },
        el("div", { class: "title" }, s.recipe.title + " "), el("span", { class: "score" }, s.score + " " + t("score")),
        reasonsList(s.reasons)))));
    openModal(
      el("h3", {}, wineTitle(w)), windowBadge(v), el("span", { class: "qty" }, " ×" + it.quantity), kv,
      el("button", { class: "primary", type: "button", disabled: it.quantity <= 0, onclick: () => guarded(() => openDrink(v)) }, t("drink")),
      el("h3", { style: "margin-top:16px" }, t("cooked")), recList,
    );
  }

  async function ensureRecipes() {
    if (!state.recipes) state.recipes = (await api("GET", "/v1/recipes")).items;
    return state.recipes;
  }

  async function openDrink(v) {
    const recipes = await ensureRecipes();
    let rating = 0;
    const stars = el("div", { class: "stars" });
    const paint = () => [...stars.children].forEach((b, i) => b.classList.toggle("on", i < rating));
    for (let i = 1; i <= 5; i++) {
      stars.appendChild(el("button", { type: "button", "aria-label": String(i), onclick: () => { rating = rating === i ? 0 : i; paint(); } }, "★"));
    }
    const note = el("textarea", { maxlength: "4000" });
    const select = el("select", {}, el("option", { value: "" }, t("none")), recipes.map((r) => el("option", { value: r.id }, r.title)));
    const form = el("form", {},
      el("h3", {}, t("drink") + ": " + wineTitle(v.wine)),
      el("div", { class: "form-row" }, el("label", {}, t("rating")), stars),
      el("div", { class: "form-row" }, el("label", {}, t("note")), note),
      el("div", { class: "form-row" }, el("label", {}, t("recipeOpt")), select),
      el("button", { class: "primary", type: "submit" }, t("save")),
    );
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      guarded(async () => {
        const body = { rating, note: note.value.trim() };
        if (select.value) body.paired_recipe_id = select.value;
        const res = await api("POST", "/v1/cellar/items/" + encodeURIComponent(v.item.id) + "/drink", body);
        closeModal();
        await loadCellar();
        const box = $("error");
        box.textContent = t("drunkOk") + res.item.quantity;
        box.classList.remove("hidden");
        setTimeout(() => box.classList.add("hidden"), 3000);
      });
    });
    openModal(form);
  }

  // ---------- recipes ----------
  async function loadRecipes() {
    state.recipes = null;
    const recipes = await ensureRecipes();
    const list = clear($("recipe-list"));
    if (!recipes.length) { list.appendChild(el("li", { class: "muted" }, t("empty"))); return; }
    for (const r of recipes) {
      list.appendChild(el("li", {},
        el("div", { class: "info", onclick: () => guarded(() => openRecipe(r)) },
          el("div", { class: "title" }, r.title),
          el("div", { class: "sub" }, [r.cuisine, r.time_minutes ? r.time_minutes + " " + t("portion") : "", "🌶".repeat(r.spiciness || 0)].filter(Boolean).join(" · ")))));
    }
  }

  async function openRecipe(r) {
    const data = await api("GET", "/v1/recipes/" + encodeURIComponent(r.id) + "/wines?limit=5");
    const list = el("ul", { class: "list" }, data.items.map((s) => el("li", {},
      el("span", { class: "dot c-" + s.bottle.wine.color }),
      el("div", { class: "info", onclick: () => guarded(() => openBottle(s.bottle.item.id)) },
        el("div", { class: "title" }, wineTitle(s.bottle.wine)),
        el("div", { class: "sub" }, (s.bottle.item.storage_location || "") + " · ×" + s.bottle.item.quantity),
        windowBadge(s.bottle), reasonsList(s.reasons)),
      el("span", { class: "score" }, s.score))));
    openModal(el("h3", {}, r.title), el("div", { class: "muted" }, t("whatOpen")), data.items.length ? list : el("p", { class: "muted" }, t("empty")));
  }

  // ---------- taste ----------
  async function loadProfile() {
    const p = await api("GET", "/v1/taste/profile");
    const box = clear($("profile"));
    const kv = el("dl", { class: "kv" });
    const row = (k, v) => { if (v !== "" && v !== undefined && v !== null) kv.append(el("dt", {}, t(k)), el("dd", {}, String(v))); };
    const scale = (n) => (n ? "●".repeat(n) + "○".repeat(Math.max(0, 5 - n)) : "—");
    row("body", scale(p.body)); row("acidity", scale(p.acidity)); row("tannin", scale(p.tannin));
    row("sweetness", scale(p.sweetness)); row("darkFruit", scale(p.fruit_dark));
    row("spice", (p.spice_tolerance || 0) + "/3");
    row("liked", (p.liked_grapes || []).join(", ") || "—");
    row("rejected", (p.rejected_grapes || []).join(", ") || "—");
    box.append(el("h3", {}, t("profile")), kv);
    if (p.notes) box.append(el("p", { class: "muted" }, p.notes));
  }

  async function loadSurvey() {
    const b = await api("GET", "/v1/taste/survey/next?size=6");
    const form = clear($("survey-form"));
    if (b.done) { form.appendChild(el("p", {}, t("surveyDone"))); return; }
    form.appendChild(el("p", { class: "muted" }, t("answered") + ": " + b.answered + " / " + b.total));
    for (const q of b.questions) {
      const name = "q-" + q.id;
      const block = el("div", { class: "question", "data-qid": q.id, "data-type": q.type }, el("div", { class: "qtext" }, q.text));
      if (q.type === "scale") {
        const sc = el("div", { class: "scale" });
        for (let i = 1; i <= 5; i++) sc.appendChild(el("label", {}, el("input", { type: "radio", name, value: String(i) }), el("span", {}, String(i))));
        block.appendChild(sc);
      } else {
        const type = q.type === "multi" ? "checkbox" : "radio";
        for (const o of q.options || []) block.appendChild(el("label", { class: "opt" }, el("input", { type, name, value: o.value }), el("span", {}, o.label)));
      }
      form.appendChild(block);
    }
    form.appendChild(el("button", { class: "primary", type: "submit" }, t("surveyNext")));
  }

  function submitSurvey(e) {
    e.preventDefault();
    const answers = [];
    $("survey-form").querySelectorAll(".question").forEach((block) => {
      const values = [...block.querySelectorAll("input:checked")].map((i) => i.value);
      if (values.length) answers.push({ question_id: block.dataset.qid, values });
    });
    if (!answers.length) { showError(new Error(t("noAnswers"))); return; }
    guarded(async () => {
      await api("POST", "/v1/taste/survey/answers", { answers });
      await Promise.all([loadProfile(), loadSurvey()]);
      window.scrollTo(0, 0);
    });
  }

  // ---------- chat ----------
  function addMessage(role, text, bottles) {
    const log = $("chat-log");
    const titles = {};
    (bottles || []).forEach((b) => { titles[b.item_id] = b.title; });
    // Replace [item:id] markers with the bottle title (plain text). If the model already wrote
    // the title right before the marker (the mock does), just drop the marker to avoid duplicates.
    const raw = String(text);
    const clean = raw.replace(/\s*\[item:([A-Za-z0-9_\-]+)\]/g, (m, id, offset) => {
      const title = titles[id];
      if (!title) return "";
      return raw.slice(0, offset).trimEnd().endsWith(title) ? "" : " «" + title + "»";
    });
    const msg = el("div", { class: "msg " + role }, clean);
    if (bottles && bottles.length) {
      msg.appendChild(el("div", { class: "bottle-chips" }, bottles.map((b) => el("span", {
        class: "bottle-chip", onclick: () => guarded(() => openBottle(b.item_id)),
      }, "🍷 " + b.title + (b.storage_location ? " · " + b.storage_location : "") + " · ×" + b.quantity))));
    }
    log.appendChild(msg);
    msg.scrollIntoView({ behavior: "smooth", block: "end" });
  }

  function submitChat(e) {
    e.preventDefault();
    const input = $("chat-input");
    const message = input.value.trim();
    if (!message) return;
    input.value = "";
    addMessage("user", message);
    guarded(async () => {
      const res = await api("POST", "/v1/chat", { message, conversation_id: state.conversationId });
      state.conversationId = res.conversation_id;
      addMessage("assistant", res.answer, res.bottles);
    });
  }

  // ---------- navigation ----------
  function switchTab(tab) {
    state.tab = tab;
    document.querySelectorAll("#tabs button").forEach((b) => b.classList.toggle("active", b.dataset.tab === tab));
    ["cellar", "recipes", "taste", "chat"].forEach((n) => $("tab-" + n).classList.toggle("hidden", n !== tab));
    if (tab === "cellar") guarded(loadCellar);
    if (tab === "recipes") guarded(loadRecipes);
    if (tab === "taste") guarded(() => Promise.all([loadProfile(), loadSurvey()]));
    if (tab === "chat" && !$("chat-log").children.length) addMessage("assistant", t("hello"));
  }

  function showApp(loggedIn) {
    $("login").classList.toggle("hidden", loggedIn);
    $("main").classList.toggle("hidden", !loggedIn);
    $("tabs").classList.toggle("hidden", !loggedIn);
    $("logout-btn").classList.toggle("hidden", !loggedIn);
  }

  function logout() {
    state.token = "";
    state.recipes = null;
    state.conversationId = "";
    localStorage.removeItem("terroir.token");
    clear($("chat-log"));
    showApp(false);
  }

  async function login(token) {
    state.token = token;
    await api("GET", "/v1/me"); // validates the token
    localStorage.setItem("terroir.token", token);
    showApp(true);
    switchTab("cellar");
  }

  // ---------- wiring ----------
  function init() {
    applyI18n();
    $("login-form").addEventListener("submit", (e) => {
      e.preventDefault();
      const tok = $("token-input").value.trim();
      if (tok) guarded(() => login(tok));
    });
    $("logout-btn").addEventListener("click", logout);
    $("lang-btn").addEventListener("click", () => {
      state.lang = state.lang === "ru" ? "en" : "ru";
      localStorage.setItem("terroir.lang", state.lang);
      applyI18n();
      state.recipes = null;
      if (state.token) switchTab(state.tab);
    });
    document.querySelectorAll("#tabs button").forEach((b) => b.addEventListener("click", () => switchTab(b.dataset.tab)));
    $("ready-chip").addEventListener("click", () => {
      state.ready = !state.ready;
      $("ready-chip").classList.toggle("active", state.ready);
      guarded(loadCellar);
    });
    let timer;
    $("search").addEventListener("input", (e) => {
      clearTimeout(timer);
      timer = setTimeout(() => { state.q = e.target.value.trim(); guarded(loadCellar); }, 300);
    });
    $("survey-form").addEventListener("submit", submitSurvey);
    $("chat-form").addEventListener("submit", submitChat);
    $("modal-close").addEventListener("click", closeModal);
    $("modal").addEventListener("click", (e) => { if (e.target === $("modal")) closeModal(); });

    if (state.token) {
      showApp(true);
      guarded(() => login(state.token));
    } else {
      showApp(false);
    }
    if ("serviceWorker" in navigator && (location.protocol === "https:" || location.hostname === "localhost")) {
      navigator.serviceWorker.register("sw.js").catch(() => {});
    }
  }

  document.addEventListener("DOMContentLoaded", init);
})();
