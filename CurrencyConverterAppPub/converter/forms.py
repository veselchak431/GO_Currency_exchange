from flask_wtf import FlaskForm
from wtforms import DateField, FloatField, SelectField, SubmitField
from wtforms.validators import DataRequired
import requests


class InputDataForms(FlaskForm):
    url = 'http://127.0.0.1:8080/currency/all'
    response = requests.get(url)
    curr = response.json()
    curr = list(curr)

    choice = []
    for i in curr:
        tpl = (i['name'], i['name'])
        choice.append(tpl)
    choice.sort()
    currency = SelectField('Валюта', choices=choice)
    quantity = FloatField(f'Сколко рублей вы готовы потратить?', validators=[DataRequired()])
    submit = SubmitField('Посчитать')
