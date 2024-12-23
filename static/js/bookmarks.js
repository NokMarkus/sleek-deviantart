document.addEventListener("DOMContentLoaded", () => {
  const bookmarkButtons = document.querySelectorAll(".bookmark-button");

  bookmarkButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const isBookmarked = button.classList.contains("bookmarked");

      const bookmark = {
        Title: button.dataset.title,
        Link: button.dataset.link,
        Description: button.dataset.description,
        Image: button.dataset.image,
      };

      const url = isBookmarked ? "/removeBookmark" : "/addBookmark";

      fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(bookmark),
      })
        .then((response) => {
          if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
          }
          return response.json();
        })
        .then((data) => {
          showToast(data.message);

          if (isBookmarked) {
            button.classList.remove("bookmarked");
            button.textContent = "Bookmark";
          } else {
            button.classList.add("bookmarked");
            button.textContent = "Unbookmark";
          }
        })
        .catch((error) => {
          console.error(
            `Error ${isBookmarked ? "removing" : "adding"} bookmark:`,
            error
          );
          showToast("An error occurred while processing the bookmark.");
        });
    });
  });

  function showToast(message) {
    const toast = document.createElement("div");
    toast.className = "toast";
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => {
      toast.classList.add("fade-out");
      toast.addEventListener("transitionend", () => toast.remove());
    }, 3000);
  }
});
