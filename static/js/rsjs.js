function overflowMenu(subtree = document) {
    // Find every overflow menu root within the provided subtree
    subtree.querySelectorAll("[data-rs-overflow-menu]").forEach(menuRoot => {
        // Avoid rebinding listeners on elements we've already initialized
        if (menuRoot.__overflowBound) return;
        menuRoot.__overflowBound = true;

        const button =
            menuRoot.querySelector("[aria-haspopup]") ||
            menuRoot.querySelector("button");

        const popover = menuRoot.querySelector("[data-popover]");
        const menu = popover
            ? popover.querySelector("[role=menu]")
            : menuRoot.querySelector("[role=menu]");

        if (!button || !popover || !menu) {
            // Nothing to wire up if essential parts are missing
            return;
        }

        const items = Array.from(menu.querySelectorAll("[role=menuitem]"));

        // Make items non tabbable initially for roving tabindex pattern
        items.forEach(item => item.setAttribute("tabindex", -1));

        const isOpen = () =>
            popover.getAttribute("aria-hidden") === "false" &&
            !popover.hasAttribute("hidden");

        function openMenu() {
            popover.removeAttribute("hidden");
            popover.setAttribute("aria-hidden", "false");
            button.setAttribute("aria-expanded", "true");
            (items[0] || menu).focus();
        }

        function closeMenu() {
            popover.setAttribute("hidden", "");
            popover.setAttribute("aria-hidden", "true");
            button.setAttribute("aria-expanded", "false");
        }

        function toggleMenu(open = !isOpen()) {
            if (open) {
                openMenu();
            } else {
                closeMenu();
            }
        }

        // Initialize to closed
        closeMenu();

        // Button click toggles
        button.addEventListener("click", (e) => {
            e.stopPropagation();
            toggleMenu();
        });

        // Close when focus leaves the whole menu root
        menuRoot.addEventListener("focusout", (e) => {
            const next = e.relatedTarget;
            if (!next || !menuRoot.contains(next)) {
                closeMenu();
            }
        });

        // Click-away to close
        const clickAway = (event) => {
            if (!menuRoot.isConnected) {
                document.removeEventListener("click", clickAway);
                return;
            }
            if (!menuRoot.contains(event.target)) {
                closeMenu();
            }
        };
        document.addEventListener("click", clickAway);

        // Keyboard interactions within the menu
        const currentIndex = () => {
            const idx = items.indexOf(document.activeElement);
            return idx === -1 ? 0 : idx;
        };

        menu.addEventListener("keydown", e => {
            if (e.key === "ArrowUp") {
                items[Math.max(0, currentIndex() - 1)]?.focus();
                e.preventDefault();
            } else if (e.key === "ArrowDown") {
                items[Math.min(items.length - 1, currentIndex() + 1)]?.focus();
                e.preventDefault();
            } else if (e.key === "Escape") {
                closeMenu();
                button.focus();
            } else if (e.key === "Tab") {
                closeMenu();
            }
        });
    });
}

// Run on initial DOM ready
// document.addEventListener("DOMContentLoaded", () => {
//     overflowMenu(document);
// });

// When htmx loads new content, initialise menus inside that fragment
document.addEventListener("htmx:load", (e) => {
    overflowMenu(e.target);
});

// After any htmx request has fully settled (including hx-swap="outerHTML"),
// rescan the real current document to catch newly replaced roots like <body>.
// document.addEventListener("htmx:afterSettle", () => {
//     overflowMenu(document);
// });


// sweetalert2: modal dialogue code
function sweetConfirm(elt, config) {
    Swal.fire(config).then((result) => {
        if (result.isConfirmed) {
            elt.dispatchEvent(new Event('confirmed'));
        }
    })
}