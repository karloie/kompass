(function () {
	const input = document.getElementById("tree-filter");
	const clearBtn = document.getElementById("tree-filter-clear");
	const namespaceSelect = document.getElementById("namespace-select");
	const root = document.getElementById("tree-root");
	const empty = document.getElementById("tree-empty");
	if (!input || !root || !empty) return;

	const nodes = Array.from(root.querySelectorAll("li"));
	const topNodes = Array.from(root.children).filter(function (el) { return el.tagName === "LI"; });
	const nodeLabels = Array.from(root.querySelectorAll(".node"));
	const nodeChildren = new Map();
	for (const node of nodes) {
		const branch = node.querySelector(":scope > ul");
		nodeChildren.set(node, branch ? Array.from(branch.children).filter(function (el) { return el.tagName === "LI"; }) : []);
	}

	let lastSyncedQuery = new URLSearchParams(window.location.search).get("q") || "";

	function escapeHTML(s) {
		return s.replace(/[&<>"']/g, function (ch) {
			if (ch === "&") return "&amp;";
			if (ch === "<") return "&lt;";
			if (ch === ">") return "&gt;";
			if (ch === '"') return "&quot;";
			return "&#39;";
		});
	}

	function updateToggle(node) {
		const btn = node.querySelector(":scope > .row > .toggle");
		if (!btn || btn.disabled) return;
		btn.textContent = node.classList.contains("collapsed") ? "▶" : "▼";
	}

	function setCollapsed(node, collapsed) {
		node.classList.toggle("collapsed", collapsed);
		updateToggle(node);
	}

	function escapeRegExp(s) {
		return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
	}

	function globToRegExp(glob) {
		let out = "^";
		for (const ch of glob) {
			if (ch === "*") {
				out += ".*";
			} else if (ch === "?") {
				out += ".";
			} else {
				out += escapeRegExp(ch);
			}
		}
		out += "$";
		return new RegExp(out, "i");
	}

	function buildMatcher(rawQuery) {
		const terms = rawQuery.split(/\s+/).filter(Boolean).map(function (t) {
			let negated = false;
			if (t.startsWith("!")) {
				negated = true;
				t = t.slice(1);
			}
			const wildcard = t.indexOf("*") !== -1 || t.indexOf("?") !== -1;
			return {
				negated: negated,
				raw: t,
				wildcard: wildcard,
				re: wildcard && t ? globToRegExp(t) : null,
				lower: t.toLowerCase()
			};
		}).filter(function (t) { return t.raw.length > 0; });

		const positives = terms.filter(function (t) { return !t.negated; });
		const negatives = terms.filter(function (t) { return t.negated; });
		const highlight = positives.find(function (t) { return !t.wildcard; }) || null;

		return {
			hasTerms: terms.length > 0,
			highlight: highlight ? highlight.lower : "",
			test: function (label) {
				const lower = label.toLowerCase();
				for (const term of positives) {
					if (term.wildcard) {
						if (!term.re.test(label)) return false;
					} else if (lower.indexOf(term.lower) === -1) {
						return false;
					}
				}
				for (const term of negatives) {
					if (term.wildcard) {
						if (term.re.test(label)) return false;
					} else if (lower.indexOf(term.lower) !== -1) {
						return false;
					}
				}
				return positives.length > 0 || negatives.length > 0;
			}
		};
	}

	function highlightLabels(query) {
		for (const el of nodeLabels) {
			const raw = el.getAttribute("data-label") || "";
			if (!query) {
				el.textContent = raw;
				continue;
			}
			const lower = raw.toLowerCase();
			let out = "";
			let i = 0;
			while (i < raw.length) {
				const idx = lower.indexOf(query, i);
				if (idx === -1) {
					out += escapeHTML(raw.slice(i));
					break;
				}
				out += escapeHTML(raw.slice(i, idx));
				out += "<mark>" + escapeHTML(raw.slice(idx, idx + query.length)) + "</mark>";
				i = idx + query.length;
			}
			el.innerHTML = out;
		}
	}

	function matchNode(node, matcher, queryActive) {
		const label = node.getAttribute("data-label") || "";
		const searchText = node.getAttribute("data-search") || label;
		const children = nodeChildren.get(node) || [];
		let childMatched = false;
		for (const child of children) {
			if (matchNode(child, matcher, queryActive)) childMatched = true;
		}
		const selfMatched = !queryActive || matcher.test(searchText);
		const visible = selfMatched || childMatched;
		node.hidden = !visible;
		if (queryActive && visible && childMatched) {
			setCollapsed(node, false);
		}
		return visible;
	}

	function syncURL(value) {
		if (value === lastSyncedQuery) return;
		const url = new URL(window.location.href);
		if (value) {
			url.searchParams.set("q", value);
		} else {
			url.searchParams.delete("q");
		}
		history.replaceState(null, "", url.toString());
		lastSyncedQuery = value;
	}

	function renderFiltered() {
		const rawQuery = input.value.trim();
		const matcher = buildMatcher(rawQuery);
		const queryActive = matcher.hasTerms;
		let visibleTop = 0;
		if (clearBtn) {
			clearBtn.hidden = rawQuery.length === 0;
		}
		highlightLabels(matcher.highlight);
		for (const node of nodes) {
			if (!queryActive) {
				setCollapsed(node, node.getAttribute("data-user-collapsed") === "true");
			}
		}
		for (const node of topNodes) {
			if (matchNode(node, matcher, queryActive)) {
				visibleTop++;
			}
		}
		empty.hidden = visibleTop > 0;
		syncURL(rawQuery);
	}

	root.addEventListener("click", function (event) {
		const btn = event.target.closest(".toggle");
		if (!btn || btn.disabled) return;
		const li = btn.closest("li");
		if (!li) return;
		const collapsed = !li.classList.contains("collapsed");
		li.setAttribute("data-user-collapsed", collapsed ? "true" : "false");
		setCollapsed(li, collapsed);
	});

	const initialQ = new URLSearchParams(window.location.search).get("q") || "";
	if (initialQ) {
		input.value = initialQ;
	}

	if (namespaceSelect) {
		namespaceSelect.addEventListener("change", function () {
			const url = new URL(window.location.href);
			const nextNamespace = namespaceSelect.value;
			url.searchParams.delete("selector");
			if (nextNamespace) {
				url.searchParams.set("namespace", nextNamespace);
			} else {
				url.searchParams.delete("namespace");
			}

			const currentQuery = input.value.trim();
			if (currentQuery) {
				url.searchParams.set("q", currentQuery);
			} else {
				url.searchParams.delete("q");
			}

			window.location.assign(url.toString());
		});
	}

	let renderDebounceTimer = 0;
	input.addEventListener("input", function () {
		window.clearTimeout(renderDebounceTimer);
		renderDebounceTimer = window.setTimeout(renderFiltered, 60);
	});

	if (clearBtn) {
		clearBtn.addEventListener("click", function () {
			input.value = "";
			renderFiltered();
			input.focus();
		});
	}

	renderFiltered();
})();
