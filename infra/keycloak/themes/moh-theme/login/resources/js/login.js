document.addEventListener("click", function (event) {
  const toggle = event.target.closest("[data-password-toggle]");

  if (!toggle) {
    return;
  }

  event.preventDefault();

  const targetId = toggle.getAttribute("data-password-target") || "password";
  const password = document.getElementById(targetId);

  if (!password) {
    return;
  }

  const isPassword = password.getAttribute("type") === "password";
  password.setAttribute("type", isPassword ? "text" : "password");
  toggle.textContent = isPassword ? "Hide" : "Show";
  toggle.setAttribute("aria-label", isPassword ? "Hide password" : "Show password");
});
