# POSIX ENV hook for sh, dash, ash, ksh, and mksh. Source the user's ENV
# file, put ENV back, then prefix PS1. The tag is $MYWT_PROMPT_TAG.
# Remember the temp dir before the user's ENV file can change TMPDIR.
__mywt_tmpdir=${TMPDIR:-/tmp}
while [ "${__mywt_tmpdir%/}" != "$__mywt_tmpdir" ]; do
  __mywt_tmpdir=${__mywt_tmpdir%/}
done
# POSIX expands ENV (parameter expansion) before using it. A value such as
# $HOME/.shrc is not a literal path. Expand it the same way, inside quotes,
# and skip it if the value is malformed. The original text is restored below.
__mywt_env=
if [ -n "${MYWT_ORIG_ENV-}" ]; then
  eval "__mywt_env=\"$MYWT_ORIG_ENV\"" || __mywt_env=
fi
if [ -n "${__mywt_env-}" ] && [ -r "$__mywt_env" ]; then
  . "$__mywt_env"
fi
if [ -n "${MYWT_ORIG_ENV-}" ]; then
  ENV=$MYWT_ORIG_ENV
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
# This file has been read. Remove the private dir. Only a mywt-prompt-*
# directory directly in the temp dir recorded above is eligible.
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
unset MYWT_SHIM_DIR __mywt_base __mywt_parent __mywt_tmpdir __mywt_env
