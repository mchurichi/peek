const LABEL_TO_BUMP = {
  "release:patch": "patch",
  "release:minor": "minor",
  "release:major": "major",
  "skip-release": "skip",
};

const BUMP_TO_LABEL = {
  patch: "release:patch",
  minor: "release:minor",
  major: "release:major",
  skip: "skip-release",
};

const BUMP_PRIORITY = { skip: 0, patch: 1, minor: 2, major: 3 };

function parseSemver(tag) {
  const m = /^v(\d+)\.(\d+)\.(\d+)$/.exec(tag || "");
  if (!m) return null;
  return {
    major: Number(m[1]),
    minor: Number(m[2]),
    patch: Number(m[3]),
  };
}

function compareSemver(a, b) {
  if (a.major !== b.major) return a.major - b.major;
  if (a.minor !== b.minor) return a.minor - b.minor;
  return a.patch - b.patch;
}

function bumpVersion(base, bump) {
  const next = { ...base };
  if (bump === "major") {
    next.major += 1;
    next.minor = 0;
    next.patch = 0;
  } else if (bump === "minor") {
    next.minor += 1;
    next.patch = 0;
  } else {
    next.patch += 1;
  }
  return `v${next.major}.${next.minor}.${next.patch}`;
}

function latestSemver(tags) {
  const semverTags = tags.map(parseSemver).filter(Boolean).sort(compareSemver);
  return semverTags.length > 0
    ? semverTags[semverTags.length - 1]
    : { major: 0, minor: 0, patch: 0 };
}

function nextTag(tags, bump) {
  return bumpVersion(latestSemver(tags), bump);
}

function releaseLabels(labels) {
  const matchedLabels = [];
  for (const label of labels || []) {
    if (LABEL_TO_BUMP[label.name]) matchedLabels.push(label.name);
  }
  return [...new Set(matchedLabels)];
}

function conventionalTypeToBump(type, breaking) {
  if (breaking) return "major";
  if (type === "feat") return "minor";
  if (type === "fix") return "patch";
  return null;
}

function parseConventionalFromHeader(header) {
  const trimmed = (header || "").trim();
  const m = trimmed.match(/^([a-z]+)(\([^)]+\))?(!)?:\s+.+$/i);
  if (!m) return null;
  const type = m[1].toLowerCase();
  const breaking = Boolean(m[3]);
  const bump = conventionalTypeToBump(type, breaking);
  if (!bump) return null;
  return {
    bump,
    reason: `Conventional prefix in title/header: ${m[1]}${m[3] || ""}`,
  };
}

function parseConventionalFromMessage(message) {
  const text = message || "";
  if (/BREAKING CHANGE:/i.test(text)) {
    return { bump: "major", reason: "Commit contains BREAKING CHANGE" };
  }
  const firstLine = text.split("\n")[0] || "";
  const parsed = parseConventionalFromHeader(firstLine);
  if (parsed) {
    return {
      bump: parsed.bump,
      reason: `Commit message indicates ${parsed.bump} bump`,
    };
  }
  return null;
}

function pickHigher(a, b) {
  if (!a) return b;
  if (!b) return a;
  return BUMP_PRIORITY[b.bump] > BUMP_PRIORITY[a.bump] ? b : a;
}

function suggestBump({ title, body, commits }) {
  const fromTitle =
    parseConventionalFromHeader(title || "") ||
    (/BREAKING CHANGE:/i.test(body || "")
      ? { bump: "major", reason: "PR body contains BREAKING CHANGE" }
      : null);

  if (fromTitle) {
    return { ...fromTitle, source: "PR title" };
  }

  let best = null;
  for (const c of commits || []) {
    const parsed = parseConventionalFromMessage(c.commit?.message || "");
    best = pickHigher(best, parsed);
    if (best?.bump === "major") break;
  }
  if (best) {
    return { ...best, source: "commit messages" };
  }

  return {
    bump: "patch",
    reason: "No conventional signal found; defaulting to patch",
    source: "default policy",
  };
}

module.exports = {
  LABEL_TO_BUMP,
  BUMP_TO_LABEL,
  releaseLabels,
  nextTag,
  suggestBump,
};
