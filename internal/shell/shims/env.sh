# POSIX ENV hook for sh, dash, ash, ksh, and mksh. Source the user's ENV
# file, put ENV back, then prefix PS1. The tag is $MYWT_PROMPT_TAG.
if [ -n "${MYWT_ORIG_ENV-}" ] && [ -r "$MYWT_ORIG_ENV" ]; then
  . "$MYWT_ORIG_ENV"
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
