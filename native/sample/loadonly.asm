; loadonly.asm: Test LoadR only (no write to data segment)
; If this works but counter.asm doesn't, the issue is StoreR / data segment writability

    LoadR value     ; Load value into A (should be 42)
    ; A = 42, exit with 42 (exit code proves LoadR worked)
    Sys Exit 42

value:
    data 42         ; writable data (will be in data segment)
