/*
 * Host adapter for the embedded-form primitives example.
 *
 * Everything in this file is host-owned: transport to /api/validate,
 * snapshot/cancel handling, locking, and logging. The runtime bundle never
 * fetches, submits, or stores anything. Applicability comes from the shared
 * condition contract (formgen.conditions/v1) carried by the descriptor: the
 * controller compiles it, hides inapplicable controls, follows conditional
 * requiredness, clears inapplicable values, and reports applicability on every
 * settled change. The server evaluates the same definition.
 */
(function () {
  "use strict";

  var runtime = window.FormgenRelationships;
  if (!runtime || typeof runtime.attachFormController !== "function") {
    console.warn("formgen runtime bundle missing; serve /runtime/ from formgen.RuntimeAssetsFS()");
    return;
  }

  var descriptor = JSON.parse(document.getElementById("formgen-descriptor").textContent);
  var root = document.querySelector("[data-formgen-auto-init]");
  var logView = document.getElementById("embedded-log");
  var snapshotView = document.getElementById("embedded-snapshot");
  var issuesView = document.getElementById("embedded-issues");
  var controller = null;
  var registry = null;
  var snapshot = null;
  var reference = null;
  var lockedPayload = null;

  // Immutable host reference recipe: freeze a typed snapshot the host considers
  // authoritative (for example the last confirmed answers). The runtime never
  // mutates it; Confirm/Amend policy stays with the host.
  function deepFreeze(value) {
    if (value && typeof value === "object" && !Object.isFrozen(value)) {
      Object.freeze(value);
      Object.keys(value).forEach(function (key) { deepFreeze(value[key]); });
    }
    return value;
  }

  // Values are only ever shown inside this page; nothing is logged elsewhere.
  function say(message) {
    var line = document.createElement("li");
    line.textContent = message;
    logView.appendChild(line);
    logView.scrollTop = logView.scrollHeight;
  }

  function show(view, value) {
    view.textContent = JSON.stringify(value, null, 2);
  }

  // Browser half of the Go-registered "note-counter" extension control. The Go
  // component renders the control with data-component="note-counter"; this
  // factory adds the live counter and returns its teardown.
  runtime.registerComponent("note-counter", function (context) {
    var input = context.element.querySelector("input, textarea");
    var counter = context.element.querySelector("[data-note-counter]");
    if (!input || !counter) {
      return undefined;
    }
    var update = function () {
      var limit = input.maxLength > 0 ? String(input.maxLength) : "-";
      counter.textContent = Array.from(input.value).length + " / " + limit;
    };
    input.addEventListener("input", update);
    update();
    return function () {
      input.removeEventListener("input", update);
    };
  });

  function describeApplicability(applicable) {
    var inactive = Object.keys(applicable || {}).filter(function (path) { return !applicable[path]; });
    return inactive.length ? "inapplicable: " + inactive.join(", ") : "all rules applicable";
  }

  function present(issues, label) {
    var presented = controller.setIssues(issues);
    show(issuesView, issues.map(function (issue) {
      return { code: issue.code, path: issue.path, message: issue.message };
    }));
    say(label + ": " + issues.length + " issue(s), " + presented.field.length + " inline, " + presented.summary.length + " in summary");
    if (issues.length > 0) {
      var focus = controller.focusIssue(issues[0]);
      say("focus: " + (focus.target || "none") + (focus.path ? " (" + focus.path + ")" : ""));
    }
  }

  function requireController() {
    if (!controller) {
      say("controller destroyed; reload the page to start again");
      return false;
    }
    return true;
  }

  root.addEventListener("submit", function (event) {
    event.preventDefault();
    say("native submit intercepted; transport stays host-owned");
  });

  var actions = {
    hydrate: function () {
      controller.setValues(
        { enabled: true, level: 4, colors: ["blue"], note: "Hello", contact: { email: "host@example.com" } },
        { notify: false }
      );
      reference = deepFreeze(controller.snapshot());
      say("hydrated sample values silently (no onChange callbacks); immutable reference captured; " + describeApplicability(controller.effective().applicable));
    },
    reference: function () {
      if (!reference) {
        say("hydrate first to capture the reference");
        return;
      }
      var problems = controller.restore(reference, { notify: false });
      say("reference restored (" + problems.length + " skipped path(s)); the frozen reference itself is unchanged");
    },
    snapshot: function () {
      // Typed snapshot with explicit presence: absent vs false, null vs "", [] vs missing.
      snapshot = Object.freeze(controller.snapshot());
      show(snapshotView, snapshot);
      say("typed snapshot captured (" + Object.keys(snapshot.presence).length + " paths, " + snapshot.issues.length + " issue(s))");
    },
    edit: function () {
      controller.setValues({ enabled: false, note: "edited" }, { notify: true });
      say("simulated edit settled into one notification; level cleared by the clear policy");
    },
    cancel: function () {
      if (!snapshot) {
        say("capture a snapshot first");
        return;
      }
      var problems = controller.restore(snapshot, { notify: false });
      say("cancel: typed snapshot restored (presence intact, " + problems.length + " skipped path(s)); " + describeApplicability(controller.effective().applicable));
    },
    validateColors: function () {
      present(controller.validate({ paths: ["colors"] }), "scoped validation (colors)");
    },
    validateAll: function () {
      present(controller.validate(), "full validation");
    },
    server: function () {
      // Send the effective projection: inactive values are already excluded.
      var payload = controller.effective().values;
      fetch("/api/validate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      })
        .then(function (response) { return response.json(); })
        .then(function (result) {
          if (!controller) {
            say("server response ignored after destroy");
            return;
          }
          present(result.issues || [], "server issues");
        })
        .catch(function (error) {
          say("server request failed: " + error);
        });
    },
    clear: function () {
      controller.clearIssues();
      show(issuesView, []);
      say("issues cleared; values preserved");
    },
    lock: function () {
      root.querySelectorAll("input, select, textarea, button").forEach(function (control) {
        if (!control.closest("[data-embedded-actions]")) {
          control.disabled = true;
        }
      });
      // The presence store keeps the last known typed values, so a snapshot taken
      // after locking still carries the answers the raw collection now omits.
      lockedPayload = controller.snapshot();
      show(snapshotView, { locked: lockedPayload.locked, values: lockedPayload.values, raw: controller.getValues() });
      say("controls locked; typed snapshot still carries " + lockedPayload.locked.length + " locked value(s) (C15)");
    },
    destroy: function () {
      controller.destroy();
      controller = null;
      registry = null;
      say("controller and registry destroyed; late results cannot reach this root");
    },
  };

  document.querySelectorAll("[data-embedded-action]").forEach(function (button) {
    button.addEventListener("click", function () {
      var name = button.getAttribute("data-embedded-action");
      if (!actions[name]) {
        return;
      }
      if (name !== "destroy" && !requireController()) {
        return;
      }
      if (name === "destroy" && !controller) {
        return;
      }
      actions[name]();
    });
  });

  runtime
    .initFormgenRoot(root)
    .then(function (initialized) {
      registry = initialized;
      controller = runtime.attachFormController(root, { registry: registry, model: descriptor.form });
      var diagnostics = controller.conditionDiagnostics();
      if (diagnostics.length) {
        say("condition definition rejected: " + diagnostics.map(function (d) { return d.code + " at " + d.path; }).join("; "));
      }
      controller.onChange(function (values, event, meta) {
        say("change origin=" + meta.origin + " paths=" + meta.paths.join(",") + "; " + describeApplicability(meta.applicable));
      });
      say("attached controller with model " + descriptor.version + " (runtime " + runtime.RUNTIME_VERSION + "); " + describeApplicability(controller.effective().applicable));
      say("engine: " + navigator.userAgent);
    })
    .catch(function (error) {
      say("initialization failed: " + error);
    });
})();
