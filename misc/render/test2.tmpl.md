# Foobar

{{ $lineItems := (program "sqleton" (verbs "ttc" "orders" "line-items") "--print-query") }}

foofoo

{{ run $lineItems "--from" "2023-01-03" }}
