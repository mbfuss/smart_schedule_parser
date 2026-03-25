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
                if not table:
                    continue

                # Нормализуем ширину всех строк до максимальной.
                # pdfplumber может возвращать строки разной длины внутри одной таблицы,
                # а страницы — таблицы с разным числом колонок.
                # Короткие строки дополняем "Merged", а колонкам присваиваем
                # целочисленные имена — тогда pd.concat не падает с
                # InvalidIndexError из-за дублирующихся или несовпадающих индексов.
                max_cols = max(len(row) for row in table)
                padded = []
                for row in table:
                    new_row = [("Merged" if cell is None else cell) for cell in row]
                    new_row += ["Merged"] * (max_cols - len(new_row))
                    padded.append(new_row)

                df = pd.DataFrame(padded, columns=range(max_cols))
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
