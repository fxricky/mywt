# POSIX ENV hook for sh, dash, ash, ksh, and mksh. Source the user's ENV
# file, put ENV back, then prefix PS1. The tag is $MYWT_PROMPT_TAG.
# Remember the temp dir before the user's ENV file can change TMPDIR.
__mywt_tmpdir=${TMPDIR:-/tmp}
while [ "${__mywt_tmpdir%/}" != "$__mywt_tmpdir" ]; do
  __mywt_tmpdir=${__mywt_tmpdir%/}
done
# Restore the literal ENV and drop the shim directory before any expansion.
# A later eval must not be able to skip this, and a quote in the value must
# not be able to run a command.
if [ -n "${MYWT_ORIG_ENV-}" ]; then
  ENV="$MYWT_ORIG_ENV"
  export ENV
else
  unset ENV
fi
__mywt_base=${MYWT_SHIM_DIR-}
__mywt_base=${__mywt_base##*/}
__mywt_parent=${MYWT_SHIM_DIR-}
__mywt_parent=${__mywt_parent%/*}
case $__mywt_base in
  mywt-prompt-*)
    case $__mywt_parent in
      "$__mywt_tmpdir")
        command rm -rf -- "$MYWT_SHIM_DIR"
        ;;
    esac
    ;;
esac
# Expand the way the shell expands ENV: $VAR, ${VAR}, and $(...) still run.
# Backslash and double quote are escaped first so a " cannot break out of
# the quoted eval. The expansion runs in a subshell; dash treats an eval
# syntax error as fatal to a sourced script, and the subshell keeps that
# from skipping the prompt tag below.
__mywt_env=
if [ -n "${MYWT_ORIG_ENV-}" ]; then
  __mywt_escaped=
  __mywt_rest="$MYWT_ORIG_ENV"
  while [ -n "$__mywt_rest" ]; do
    __mywt_c=${__mywt_rest%"${__mywt_rest#?}"}
    __mywt_rest=${__mywt_rest#?}
    case $__mywt_c in
      \\|'"') __mywt_escaped="${__mywt_escaped}\\${__mywt_c}" ;;
      *) __mywt_escaped="${__mywt_escaped}${__mywt_c}" ;;
    esac
  done
  __mywt_env=$( (eval "printf '%s' \"$__mywt_escaped\"") 2>/dev/null ) || __mywt_env=
fi
if [ -n "${__mywt_env-}" ] && [ -r "$__mywt_env" ]; then
  . "$__mywt_env"
fi
if [ -n "${MYWT_ORIG_ENV-}" ]; then
  ENV="$MYWT_ORIG_ENV"
  export ENV
else
  unset ENV
fi
if [ -n "${MYWT_PARENT_PROMPT_TAG-}" ]; then
  case ${PS1-} in
    "$MYWT_PARENT_PROMPT_TAG"*) PS1=${PS1#"$MYWT_PARENT_PROMPT_TAG"} ;;
  esac
fi
case ${PS1-} in
  "$MYWT_PROMPT_TAG"*) ;;
  *) PS1="${MYWT_PROMPT_TAG}${PS1-\$ }" ;;
esac
unset MYWT_SHIM_DIR __mywt_base __mywt_parent __mywt_tmpdir __mywt_env __mywt_escaped __mywt_rest __mywt_c
