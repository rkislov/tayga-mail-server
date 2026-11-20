#!/usr/bin/env python3
"""Regenerate embedded office starters (development only).

Requires python-docx, openpyxl and python-pptx. The server has no Python dependency.
Templates contain no user data, macros, remote links or embedded objects.
"""
from pathlib import Path
from datetime import datetime, timezone
from io import BytesIO
from zipfile import ZipFile, ZipInfo, ZIP_DEFLATED
from docx import Document
from docx.shared import Pt
from openpyxl import Workbook
from pptx import Presentation

ROOT = Path(__file__).resolve().parents[1] / 'internal/httpapi/file_templates'
ROOT.mkdir(parents=True, exist_ok=True)
STAMP = datetime(2026, 11, 5, tzinfo=timezone.utc)

def save(package, name):
    props = getattr(package, 'core_properties', getattr(package, 'properties', None))
    for attr, value in [('author', 'Tayga'), ('creator', 'Tayga'), ('last_modified_by', ''), ('lastModifiedBy', ''), ('created', STAMP), ('modified', STAMP)]:
        if hasattr(props, attr):
            setattr(props, attr, value)
    data = BytesIO()
    package.save(data)
    # Stable archive timestamps and entry order keep binary diffs reproducible.
    with ZipFile(data) as source, ZipFile(ROOT / name, 'w', ZIP_DEFLATED) as target:
        for entry in sorted(source.namelist()):
            info = ZipInfo(entry, (2026, 11, 5, 0, 0, 0))
            info.compress_type = ZIP_DEFLATED
            target.writestr(info, source.read(entry))

doc = Document()
doc.styles['Normal'].font.name = 'Arial'
doc.styles['Normal'].font.size = Pt(11)
doc.add_paragraph()
save(doc, 'document.docx')

minutes = Document()
minutes.add_heading('Протокол встречи', 0)
for title, body in [('Дата и место', 'Укажите дату и место встречи.'), ('Участники', 'Перечислите участников.'), ('Повестка', 'Добавьте вопросы для обсуждения.'), ('Решения', 'Зафиксируйте принятые решения.'), ('Следующие действия', 'Укажите действия, ответственных и сроки.')]:
    minutes.add_heading(title, 1)
    minutes.add_paragraph(body)
save(minutes, 'meeting-minutes.docx')

book = Workbook()
book.active.title = 'Лист 1'
save(book, 'spreadsheet.xlsx')

slides = Presentation()
slide = slides.slides.add_slide(slides.slide_layouts[0])
slide.shapes.title.text = 'Название презентации'
slide.placeholders[1].text = 'Добавьте подзаголовок'
save(slides, 'presentation.pptx')
