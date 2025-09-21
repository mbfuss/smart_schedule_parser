#!/usr/bin/env python3
import sys

import pandas as pd
import pdfplumber


def pdf_to_csv_stdout(pdf_path: str):
    all_tables = []
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            for table in tables:
                cleaned = []
                for row in table:
                    new_row = []
                    for cell in row:
                        if cell is None:
                            new_row.append("Merged")  # Объединенные ячейки
                        else:
                            new_row.append(cell)

                    cleaned.append(new_row)

                df = pd.DataFrame(cleaned[1:], columns=cleaned[0])
                all_tables.append(df)

    if not all_tables:
        print("No tables found", file=sys.stderr)
        sys.exit(1)

    result = pd.concat(all_tables, ignore_index=True)
    result.to_csv(sys.stdout, index=False)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: pdf2csv.py <path-to-pdf>", file=sys.stderr)
        sys.exit(1)

    pdf_path = sys.argv[1]
    pdf_to_csv_stdout(pdf_path)
