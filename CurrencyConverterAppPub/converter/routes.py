from flask import render_template, flash
import requests
from converter import app
from converter.forms import InputDataForms


@app.route('/', methods=['GET', 'POST'])
def home():
    forms = InputDataForms()
    if forms.validate_on_submit():
        currency = forms.currency.data
        quantity = forms.quantity.data
        
        base_url = 'http://127.0.0.1:8080/currency/latest?currency='

        url = base_url + currency
        response = requests.get(url)

        if response.ok is False:
            flash(f'Error: {response.status_code}')
            flash(f"{response.json()['error']}")
        else:
            data = response.json()
            total = quantity/data['exchange_to_rub']
            flash("{} рублей = {:.2f} {}".format(quantity, total, currency))

    return render_template('home.html', form=forms)
