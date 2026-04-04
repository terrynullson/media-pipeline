(() => {
    const root = document.querySelector("[data-details-controller]");
    const buttons = Array.from(document.querySelectorAll("[data-transcript-tab]"));
    const panels = Array.from(document.querySelectorAll("[data-transcript-panel]"));
    const deleteForm = document.querySelector(".transcript-delete-form");

    function activateTab(target) {
        buttons.forEach((button) => {
            const isActive = button.dataset.transcriptTab === target;
            button.classList.toggle("is-active", isActive);
            button.setAttribute("aria-selected", isActive ? "true" : "false");
        });
        panels.forEach((panel) => {
            panel.hidden = panel.dataset.transcriptPanel !== target;
        });
    }

    buttons.forEach((button) => {
        button.addEventListener("click", () => {
            activateTab(button.dataset.transcriptTab);
        });
    });

    if (deleteForm) {
        deleteForm.addEventListener("submit", (event) => {
            const confirmed = window.confirm("Удалить этот файл? Будут удалены расшифровка, задачи и сохранённые файлы.");
            if (!confirmed) {
                event.preventDefault();
            }
        });
    }

    activateTab("full-text");

    if (!root) {
        return;
    }

    const players = Array.from(root.querySelectorAll("[data-media-player]"));
    const segments = Array.from(root.querySelectorAll("[data-segment]")).map((element) => ({
        element,
        index: Number(element.dataset.segmentIndex),
        start: Number(element.dataset.start),
        end: Number(element.dataset.end),
    })).filter((segment) => Number.isFinite(segment.start) && Number.isFinite(segment.end));
    const scrollRegion = root.querySelector("[data-transcript-scroll-region]");
    const seekables = Array.from(document.querySelectorAll("[data-seek]"));
    const playerModeButtons = Array.from(root.querySelectorAll("[data-player-mode]"));

    if (players.length === 0) {
        return;
    }

    let player = players.find((item) => !item.hidden) || players[0];

    const toleranceSec = 0.05;
    const manualScrollCooldownMS = 2500;
    let activeSegmentIndex = -1;
    let manualScrollUntil = 0;
    let scrollResetTimer = 0;

    function rememberManualScroll() {
        manualScrollUntil = window.performance.now() + manualScrollCooldownMS;
    }

    function resetAutoFollow(reason) {
        if (reason === "seek" || reason === "play") {
            manualScrollUntil = 0;
        }
    }

    function syncPlayerModeButtons(nextKind) {
        playerModeButtons.forEach((button) => {
            const isActive = button.dataset.playerMode === nextKind;
            button.classList.toggle("is-active", isActive);
            button.setAttribute("aria-pressed", isActive ? "true" : "false");
        });
    }

    async function setActivePlayer(nextKind) {
        const nextPlayer = players.find((item) => item.dataset.playerKind === nextKind);
        if (!nextPlayer || nextPlayer === player) {
            syncPlayerModeButtons(nextKind);
            return;
        }

        const previousPlayer = player;
        const resumeAt = Number.isFinite(previousPlayer.currentTime) ? previousPlayer.currentTime : 0;
        const shouldResume = !previousPlayer.paused && !previousPlayer.ended;

        previousPlayer.pause();
        previousPlayer.hidden = true;
        nextPlayer.hidden = false;
        player = nextPlayer;
        syncPlayerModeButtons(nextKind);

        if (Number.isFinite(resumeAt)) {
            player.currentTime = Math.max(0, resumeAt);
        }
        syncActiveSegment({ follow: false });

        if (shouldResume) {
            try {
                await player.play();
            } catch (_error) {
                // Keep the UI stable even if the browser blocks resumed playback.
            }
        }
    }

    function isElementVisibleWithinContainer(container, element) {
        const containerTop = container.scrollTop;
        const containerBottom = containerTop + container.clientHeight;
        const elementTop = element.offsetTop;
        const elementBottom = elementTop + element.offsetHeight;
        const topPadding = 20;
        const bottomPadding = 32;

        return elementTop >= containerTop + topPadding && elementBottom <= containerBottom - bottomPadding;
    }

    function followActiveSegment(segment) {
        if (!scrollRegion || !segment || !player || player.paused) {
            return;
        }
        if (window.performance.now() < manualScrollUntil) {
            return;
        }
        if (isElementVisibleWithinContainer(scrollRegion, segment.element)) {
            return;
        }

        const targetTop = Math.max(segment.element.offsetTop - Math.max(24, scrollRegion.clientHeight * 0.2), 0);
        scrollRegion.scrollTo({
            top: targetTop,
            behavior: "smooth",
        });
    }

    function setActiveSegment(nextIndex, options = {}) {
        if (nextIndex === activeSegmentIndex) {
            return;
        }

        if (activeSegmentIndex >= 0 && activeSegmentIndex < segments.length) {
            segments[activeSegmentIndex].element.classList.remove("is-active");
        }

        activeSegmentIndex = nextIndex;
        if (activeSegmentIndex < 0 || activeSegmentIndex >= segments.length) {
            return;
        }

        const segment = segments[activeSegmentIndex];
        segment.element.classList.add("is-active");
        if (options.follow !== false) {
            followActiveSegment(segment);
        }
    }

    function findActiveSegmentIndex(currentTime) {
        if (!Number.isFinite(currentTime)) {
            return -1;
        }

        for (let index = 0; index < segments.length; index += 1) {
            const segment = segments[index];
            const startsBeforeEnd = currentTime + toleranceSec >= segment.start;
            const endsAfterStart = currentTime < segment.end + toleranceSec;
            if (startsBeforeEnd && endsAfterStart) {
                return index;
            }
        }

        return -1;
    }

    function syncActiveSegment(options = {}) {
        setActiveSegment(findActiveSegmentIndex(player.currentTime), options);
    }

    function seekTo(seconds, source) {
        if (!Number.isFinite(seconds) || !player) {
            return;
        }

        const safeSeconds = Math.max(0, seconds);
        resetAutoFollow(source);
        player.currentTime = safeSeconds;
        syncActiveSegment({ follow: source !== "scrub" });
    }

    function onManualScroll() {
        if (scrollResetTimer) {
            window.clearTimeout(scrollResetTimer);
        }
        rememberManualScroll();
        scrollResetTimer = window.setTimeout(() => {
            scrollResetTimer = 0;
        }, manualScrollCooldownMS);
    }

    if (scrollRegion) {
        scrollRegion.addEventListener("wheel", onManualScroll, { passive: true });
        scrollRegion.addEventListener("touchmove", onManualScroll, { passive: true });
        scrollRegion.addEventListener("pointerdown", onManualScroll);
    }

    seekables.forEach((element) => {
        element.addEventListener("click", (event) => {
            event.preventDefault();
            event.stopPropagation();
            seekTo(Number(element.dataset.seek), "seek");
        });
    });

    playerModeButtons.forEach((button) => {
        button.addEventListener("click", () => {
            void setActivePlayer(button.dataset.playerMode);
        });
    });

    players.forEach((currentPlayer) => {
        currentPlayer.addEventListener("timeupdate", () => {
            if (currentPlayer !== player) {
                return;
            }
            syncActiveSegment();
        });
        currentPlayer.addEventListener("seeked", () => {
            if (currentPlayer !== player) {
                return;
            }
            resetAutoFollow("seek");
            syncActiveSegment();
        });
        currentPlayer.addEventListener("play", () => {
            if (currentPlayer !== player) {
                return;
            }
            resetAutoFollow("play");
            syncActiveSegment();
        });
        currentPlayer.addEventListener("pause", () => {
            if (currentPlayer !== player) {
                return;
            }
            syncActiveSegment({ follow: false });
        });
    });

    syncActiveSegment({ follow: false });
})();
