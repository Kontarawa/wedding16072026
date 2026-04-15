(function () {
  const cfg = (window && window.WEDDING_CONFIG) || {};
  const webhookUrl = (cfg.webhookUrl || "").trim();
  const webhookToken = (cfg.webhookToken || "").trim();
  const useWebhook = true;

  const greetingEl = document.getElementById("greeting");
  const firstInput = document.getElementById("first_name");
  const lastInput = document.getElementById("last_name");
  const form = document.getElementById("rsvp-form");
  const errEl = document.getElementById("form-error");
  const mainFlow = document.getElementById("main-flow");
  const pageRoot = document.querySelector(".page");
  const thanks = document.getElementById("thanks-panel");
  const submitOverlay = document.getElementById("submit-overlay");
  const submitBtn = form ? form.querySelector('button[type="submit"]') : null;
  const summaryGuest = document.getElementById("summary-guest");
  const summaryAttendance = document.getElementById("summary-attendance");
  const summaryTransfer = document.getElementById("summary-transfer");
  const summaryAlcohol = document.getElementById("summary-alcohol");
  const summarySubmitted = document.getElementById("summary-submitted");
  const countdownHeroEl = document.getElementById("wedding-countdown");
  const countdownThanksEl = document.getElementById("wedding-countdown-thanks");

  let isSubmitting = false;
  let lastSubmission = null;
  let countdownTimer = null;

  function buildSubmissionHash(payload) {
    const raw = [payload.first_name, payload.last_name, payload.submitted_at].map((v) => (v || "").trim()).join("|");
    let h = 0;
    for (let i = 0; i < raw.length; i++) {
      h = (h * 31 + raw.charCodeAt(i)) | 0;
    }
    return `h${Math.abs(h)}`;
  }

  function setGreeting(first, last, salutation) {
    const sal = (salutation || "").trim();
    if (sal) {
      greetingEl.textContent = sal;
      return;
    }
    const f = (first || "").trim();
    const l = (last || "").trim();
    if (f || l) {
      const name = [f, l].filter(Boolean).join(" ");
      greetingEl.textContent = `Дорогие ${name}!`;
    } else {
      greetingEl.textContent = "Дорогой Гость!";
    }
  }

  function showError(msg) {
    errEl.textContent = msg || "";
    errEl.classList.toggle("hidden", !msg);
  }

  function setSubmitting(next) {
    isSubmitting = !!next;
    if (submitBtn) submitBtn.disabled = isSubmitting;
    if (submitOverlay) {
      submitOverlay.classList.toggle("hidden", !isSubmitting);
      submitOverlay.setAttribute("aria-hidden", isSubmitting ? "false" : "true");
    }
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (isSubmitting) return;
    showError("");
    if (!webhookUrl || !webhookToken) {
      showError("Не настроена отправка анкеты. Заполните WEDDING_CONFIG.webhookUrl и WEDDING_CONFIG.webhookToken в index.html.");
      return;
    }
    const alcoholChoices = [...form.querySelectorAll('input[name="alcohol"]:checked')].map((el) => el.value);
    const payload = {
      first_name: firstInput.value.trim(),
      last_name: lastInput.value.trim(),
      attendance: form.querySelector('input[name="attendance"]:checked')?.value || "",
      transfer: form.querySelector('input[name="transfer"]:checked')?.value || "",
      alcohol: alcoholChoices.join(", "),
      submitted_at: new Date().toISOString(),
    };
    if (!payload.first_name || !payload.last_name) {
      showError("Укажите имя и фамилию.");
      return;
    }
    if (!payload.attendance || !payload.transfer || !alcoholChoices.length) {
      showError("Пожалуйста, ответьте на все вопросы.");
      return;
    }
    try {
      setSubmitting(true);
      const body = useWebhook
        ? {
            token: webhookToken,
            ...payload,
            hash: buildSubmissionHash(payload),
          }
        : payload;

      const res = await fetch(webhookUrl, {
        method: "POST",
        // Apps Script Web App часто отвечает редиректом, а fetch в браузере может
        // превратить POST в GET при follow-redirect и дать ложную ошибку.
        // Для вебхука нам важнее "доставить" запрос, чем читать ответ.
        mode: "no-cors",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      lastSubmission = payload;
      enterThanksState(payload);
    } catch (err) {
      showError("Не удалось отправить форму. Попробуйте позже или напишите нам лично.");
      console.error(err);
    } finally {
      setSubmitting(false);
    }
  });

  function safeText(v) {
    return (v ?? "").toString().trim();
  }

  function formatSubmittedAt(iso) {
    const s = safeText(iso);
    if (!s) return "—";
    const d = new Date(s);
    if (Number.isNaN(d.getTime())) return s;
    return d.toLocaleString("ru-RU", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
  }

  function renderSummary(payload) {
    const p = payload || lastSubmission;
    if (!p) return;
    const guest = [safeText(p.first_name), safeText(p.last_name)].filter(Boolean).join(" ");
    if (summaryGuest) summaryGuest.textContent = guest || "—";
    if (summaryAttendance) summaryAttendance.textContent = safeText(p.attendance) || "—";
    if (summaryTransfer) summaryTransfer.textContent = safeText(p.transfer) || "—";
    if (summaryAlcohol) summaryAlcohol.textContent = safeText(p.alcohol) || "—";
    if (summarySubmitted) summarySubmitted.textContent = formatSubmittedAt(p.submitted_at);
  }

  function enterThanksState(payload) {
    mainFlow.classList.add("hidden");
    thanks.classList.remove("hidden");
    thanks.classList.add("is-inview");
    if (pageRoot) pageRoot.classList.add("thanks-only");
    renderSummary(payload);
    startCountdown();
    thanks.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function formatDelta(ms) {
    const totalSec = Math.max(0, Math.floor(ms / 1000));
    const days = Math.floor(totalSec / 86400);
    const hours = Math.floor((totalSec % 86400) / 3600);
    const mins = Math.floor((totalSec % 3600) / 60);
    const secs = totalSec % 60;
    const pad = (n) => String(n).padStart(2, "0");
    if (days > 0) return `${days} дн ${pad(hours)}:${pad(mins)}:${pad(secs)}`;
    return `${pad(hours)}:${pad(mins)}:${pad(secs)}`;
  }

  function startCountdown() {
    // 16 июля 2026, 15:00 (локальное время браузера)
    const weddingAt = new Date(2026, 6, 16, 15, 0, 0, 0);

    if (countdownTimer) clearInterval(countdownTimer);

    const tick = () => {
      const now = new Date();
      const ms = weddingAt.getTime() - now.getTime();
      if (ms <= 0) {
        if (countdownHeroEl) countdownHeroEl.textContent = "Уже началось!";
        if (countdownThanksEl) countdownThanksEl.textContent = "Уже началось!";
        if (countdownTimer) clearInterval(countdownTimer);
        countdownTimer = null;
        return;
      }
      const txt = formatDelta(ms);
      if (countdownHeroEl) countdownHeroEl.textContent = txt;
      if (countdownThanksEl) countdownThanksEl.textContent = txt;
    };

    tick();
    countdownTimer = setInterval(tick, 1000);
  }

  const heroImg = document.querySelector(".hero-cover");
  if (heroImg) {
    heroImg.addEventListener("error", () => {
      heroImg.src =
        "data:image/svg+xml," +
        encodeURIComponent(
          `<svg xmlns='http://www.w3.org/2000/svg' width='640' height='800' viewBox='0 0 640 800'><defs><linearGradient id='g' x1='0' y1='0' x2='1' y2='1'><stop offset='0' stop-color='#e8ebe5'/><stop offset='1' stop-color='#d5ddd0'/></linearGradient></defs><rect width='640' height='800' fill='url(#g)'/><text x='50%' y='46%' dominant-baseline='middle' text-anchor='middle' fill='#4a5a44' font-family='system-ui,sans-serif' font-size='28'>Алина и Роман</text><text x='50%' y='52%' dominant-baseline='middle' text-anchor='middle' fill='#6a7a62' font-family='system-ui,sans-serif' font-size='15'>static/couple.png</text></svg>`
        );
    });
  }

  function initReveal() {
    const nodes = document.querySelectorAll(".reveal:not(#thanks-panel)");
    if (!nodes.length) return;
    if (!("IntersectionObserver" in window)) {
      nodes.forEach((el) => el.classList.add("is-inview"));
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          entry.target.classList.add("is-inview");
          io.unobserve(entry.target);
        });
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.08 }
    );
    nodes.forEach((el, i) => {
      el.style.setProperty("--d", `${Math.min(i * 45, 200)}ms`);
      io.observe(el);
    });
  }

  // Имя и фамилия всегда вводятся вручную (без автозаполнения по ссылке).
  setGreeting("", "", "");
  startCountdown();
  initReveal();
})();
