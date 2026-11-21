# Runs from fish --init-command, after config.fish. The tag is $MYWT_PROMPT_TAG.
if functions -q fish_prompt
    functions -c fish_prompt __mywt_orig_fish_prompt
    function fish_prompt
        set -l out (__mywt_orig_fish_prompt | string collect)
        if set -q MYWT_PARENT_PROMPT_TAG; and test -n "$MYWT_PARENT_PROMPT_TAG"
            if string match -q -- "$MYWT_PARENT_PROMPT_TAG*" -- $out
                set out (string sub --start (math (string length -- "$MYWT_PARENT_PROMPT_TAG") + 1) -- $out)
            end
        end
        if not string match -q -- "$MYWT_PROMPT_TAG*" -- $out
            set out "$MYWT_PROMPT_TAG$out"
        end
        printf '%s' $out
    end
else
    function fish_prompt
        printf '%s> ' $MYWT_PROMPT_TAG
    end
end
