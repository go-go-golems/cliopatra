# Foobar

{{ $lineItems := (program "sqleton" (verbs "ttc" "orders" "line-items") "--print-query") }}

```line_items_since_2023-01-03
{{ run $lineItems "--from" "2023-01-03" }}
```

```line_items_in_2022
{{ run $lineItems "--from" "2022-01-01" "--to" "2022-12-31" }}
```

```aggregate_line_items_in_2021
{{ run $lineItems "--from" "2021-01-01" "--to" "2021-12-31" "--aggregate" }}
```