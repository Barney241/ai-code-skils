#!/usr/bin/env bash
# Install skills from this repo into a target repository.
#
#   ./install.sh review /path/to/repo        copy the review skill in
#   ./install.sh --all /path/to/repo         copy every skill in
#   ./install.sh --link review /path/to/repo symlink instead (tracks this checkout)
#   ./install.sh --uninstall review /path/to/repo
#   ./install.sh --list
#
# Layout produced in the target repo:
#   .agents/skills/<name>/     the skill itself (tool-neutral source of truth)
#   .claude/skills/<name>      symlink -> ../../.agents/skills/<name>
set -euo pipefail

SRC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SRC="$SRC_ROOT/skills"

MODE=copy
ACTION=install
SELECTED=()
TARGET=""

die()  { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '  %s\n' "$*"; }

available() {
  find "$SKILLS_SRC" -mindepth 2 -maxdepth 2 -name SKILL.md -printf '%h\n' \
    | sed "s|^$SKILLS_SRC/||" | sort
}

usage() {
  sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
  printf '\nAvailable skills:\n'
  available | sed 's/^/  /'
}

while [ $# -gt 0 ]; do
  case "$1" in
    --list)      available; exit 0 ;;
    --all)       SELECTED=(__ALL__) ;;
    --link)      MODE=link ;;
    --copy)      MODE=copy ;;
    --uninstall) ACTION=uninstall ;;
    -h|--help)   usage; exit 0 ;;
    -*)          die "unknown flag: $1" ;;
    *)
      if [ -d "$SKILLS_SRC/$1" ] || [ "$1" = "__ALL__" ]; then
        SELECTED+=("$1")
      else
        TARGET="$1"
      fi
      ;;
  esac
  shift
done

[ ${#SELECTED[@]} -gt 0 ] || { usage; die "no skill named"; }
TARGET="${TARGET:-$PWD}"
[ -d "$TARGET" ] || die "target is not a directory: $TARGET"
TARGET="$(cd "$TARGET" && pwd)"

if [ "${SELECTED[0]}" = "__ALL__" ]; then
  mapfile -t SELECTED < <(available)
fi

for name in "${SELECTED[@]}"; do
  src="$SKILLS_SRC/$name"
  [ -f "$src/SKILL.md" ] || die "no SKILL.md in $src"

  dest="$TARGET/.agents/skills/$name"
  link="$TARGET/.claude/skills/$name"

  if [ "$ACTION" = uninstall ]; then
    printf 'uninstalling %s from %s\n' "$name" "$TARGET"
    [ -e "$link" ] || [ -L "$link" ] && rm -f "$link" && info "removed $link"
    if [ -L "$dest" ]; then rm -f "$dest"; info "removed symlink $dest"
    elif [ -d "$dest" ]; then rm -rf "$dest"; info "removed $dest"
    fi
    continue
  fi

  printf 'installing %s into %s (%s)\n' "$name" "$TARGET" "$MODE"
  mkdir -p "$TARGET/.agents/skills" "$TARGET/.claude/skills"

  # Replace any previous install so re-running is idempotent.
  [ -L "$dest" ] && rm -f "$dest"
  [ -d "$dest" ] && rm -rf "$dest"

  if [ "$MODE" = link ]; then
    ln -s "$src" "$dest"
    info "linked $dest -> $src"
  else
    cp -R "$src" "$dest"
    info "copied $dest"
  fi

  # Claude Code discovers skills under .claude/skills; keep it a relative
  # symlink so the target repo stays portable across machines and worktrees.
  [ -L "$link" ] && rm -f "$link"
  [ -e "$link" ] && die "$link exists and is not a symlink; move it aside first"
  ln -s "../../.agents/skills/$name" "$link"
  info "symlinked $link -> ../../.agents/skills/$name"

  if [ -f "$src/rules/project.template.md" ]; then
    info "next: copy rules/project.template.md to rules/project.md and fill in your repo's traps"
  fi
done

printf '\ndone. For editors other than Claude Code see adapters/README.md\n'
