function overflowMenu(subtree = document) {
    // Find every overflow menu root within the provided subtree
    subtree.querySelectorAll("[data-overflow-menu]").forEach(menuRoot => {
        // Avoid rebinding listeners on elements we've already initialized
        if (menuRoot.__overflowBound) return;
        menuRoot.__overflowBound = true;

        const button = menuRoot.querySelector("[aria-haspopup]") || menuRoot.querySelector("button");
        const menu = menuRoot.querySelector("[role=menu]");
        if (!button || !menu) {
            // Nothing to wire up if essential parts are missing
            return;
        }

        const items = Array.from(menu.querySelectorAll("[role=menuitem]"));
        // Ensure menu is hidden initially if not set by markup
        if (menu.hidden === undefined) {
            // some elements may not support .hidden; enforce via attribute
            menu.setAttribute("hidden", "");
        } else {
            menu.hidden = true;
        }

        // make items non tabbable for roving tabindex pattern
        items.forEach(item => item.setAttribute("tabindex", -1));

        const isOpen = () => menu.getAttribute("hidden") === null ? true : !menu.hidden ? true : false;

        function openMenu() {
            // Show menu using both property and attribute to be resilient
            menu.hidden = false;
            menu.removeAttribute("hidden");
            button.setAttribute("aria-expanded", "true");
            (items[0] || menu).focus();
        }

        function closeMenu() {
            menu.hidden = true;
            menu.setAttribute("hidden", "");
            button.setAttribute("aria-expanded", "false");
        }

        function toggleMenu(open = !isOpen()) {
            if (open) openMenu(); else closeMenu();
        }

        // Initialize to closed
        closeMenu();

        // Button click toggles
        button.addEventListener("click", () => toggleMenu());

        // Close when focus leaves the whole menu root
        menuRoot.addEventListener("focusout", (e) => {
            // relatedTarget is the element gaining focus (can be null)
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
            if (!menuRoot.contains(event.target)) closeMenu();
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

// Run on initial DOM ready and whenever HTMX swaps in content
document.addEventListener("DOMContentLoaded", () => overflowMenu(document));
document.addEventListener("htmx:load", e => overflowMenu(e.target));
document.addEventListener("htmx:afterSwap", e => overflowMenu(e.target));


// sweetalert2: modal dialogue code
function sweetConfirm(elt, config) {
    Swal.fire(config).then((result) => {
        if (result.isConfirmed) {
            elt.dispatchEvent(new Event('confirmed'));
        }
    })
}