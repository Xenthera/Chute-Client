const init = () => {
  const messages = document.getElementById("messages");
  if (messages) {
    messages.textContent = "Chute GUI Running";
  }
};

window.addEventListener("DOMContentLoaded", init);

