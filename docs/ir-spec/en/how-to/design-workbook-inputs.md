# Design workbook inputs

Define what one row represents. Expose only values that vary per row; keep fixed policy in instructions. Use order, required, enum, presentation hints, and examples. Hidden/default fields require a default value. After version creation, download and inspect the real workbook; download again after schema changes.

Recommended sequence:

1. Name the business object represented by one row.
2. Expose only row-varying inputs.
3. Put fixed requirements in step instructions.
4. Order fields and mark required values.
5. Use enum/select for controlled choices and textarea for long content.
6. Provide hints and examples for ambiguous fields.
7. Give every hidden/default field a value.

Validate the generated workbook's headers, dropdowns, help, and hidden behavior—not just the JSON.
