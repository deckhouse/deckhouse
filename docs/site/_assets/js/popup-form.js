document.addEventListener("DOMContentLoaded", function() {
  class PopupForm {
    constructor(wrapper) {
      this.wrapper = wrapper;
      this.modalAttr = this.wrapper.dataset.modalWindow;
      this.form = this.wrapper.querySelector('[data-form]');
      this.url = this.form.getAttribute('action');
      this.intro = this.wrapper.querySelector('[data-header-form]');
      this.closeBtn = this.wrapper.querySelector('[data-close-btn]')
      this.closeBg = this.wrapper.querySelector('[data-close-bg]');
      this.success = this.wrapper.querySelector('[data-success-message]');
      this.error = this.wrapper.querySelector('[data-error-message]');
      this.initializeOpenModalButton();
      this.form.addEventListener('submit', this.submitForm.bind(this));
      this.closeBtn.addEventListener('click', this.closeModal.bind(this));
      this.closeBg.addEventListener('click', this.closeModal.bind(this));

      this.preferredContact = document.querySelector('input[name="preferred_contact"]');
      this.telegramInput = document.querySelector('input[name="telegram_id"]');
      this.telegramCheckbox = document.querySelector('input[value="telegram"]');
      this.updateContactValue();
      this.initializeCheckbox();
      this.toggleTelegramInput();
      this.telegramCheckbox.addEventListener('change', this.toggleTelegramInput.bind(this));
    }

    initializeCheckbox() {
      const checkboxes = document.querySelectorAll('input[type="checkbox"]');
      checkboxes.forEach(checkbox => {
        checkbox.addEventListener('change', this.updateContactValue.bind(this));
      });
    }

    updateContactValue() {
      let selectedContacts = [];
      const checkboxes = document.querySelectorAll('input[type="checkbox"]');
      checkboxes.forEach(checkbox => {
        if(checkbox.checked) {
          selectedContacts.push(checkbox.value);
        }
      });
      this.preferredContact.value = selectedContacts.join(',');
    }

    toggleTelegramInput() {
      if(this.telegramCheckbox.checked) {
        this.telegramInput.style.display = 'block';
      } else {
        this.telegramInput.style.display = 'none';
        this.telegramInput.value = '';
      }
    }

    initializeOpenModalButton() {
      const openButtons = document.querySelectorAll(`[data-open-modal="${this.modalAttr}"]`);
      openButtons.forEach(button => {
        button.addEventListener('click', this.openModal.bind(this));
      })
    }

    submitForm(e) {
      e.preventDefault();

      const FormData = this.serializeData();
      console.log(FormData)

      const bitrixFields = {
        fields: {
          'TITLE': 'с сайта документации Deckhouse',
          'NAME': FormData.name,
          'EMAIL': FormData.email,
          'PHONE': FormData.phone,
          'COMPANY': FormData.company,
          'POST': FormData.position,
          'COMMENTS': 'Предпочтительный вид связи: ' + FormData.preferred_contact,
        }
      }

      if(this.telegramCheckbox.checked && this.telegramInput.value) {
        bitrixFields.fields.COMMENTS += '. Telegram ID: ' + this.telegramInput.value;
      }

      BX24.callMethod(
        "crm.lead.add",
        bitrixFields,
        res => {
          if (res.ok) {
            this.downloadFile();
            this.successSubmit();
          } else {
            this.errorSubmit();
          }
        }
      )
    }

    serializeData() {
      let data = new FormData(this.form);
      let serializedData = Object.fromEntries(data.entries());
      serializedData.referer_url = window.location.href;
      return serializedData;
    }

    successSubmit() {
      this.intro.style.display = 'none';
      this.success.style.display = 'block';
    }

    errorSubmit() {
      this.intro.style.display = 'none';
      this.error.style.display = 'block';
    }

    openModal(e) {
      e.preventDefault();
      this.wrapper.style.display = 'flex';
      document.addEventListener('keydown', this.closeModalOnEscape.bind(this));
    }

    closeModal(e) {
      e.preventDefault();
      this.wrapper.style.display = 'none';
      this.intro.style.display = 'block';
      this.success.style.display = 'none';
      this.error.style.display = 'none';
    }

    closeModalOnEscape(e) {
      if (e.key === 'Escape') {
        this.wrapper.style.display = 'none';
        this.intro.style.display = 'block';
        this.success.style.display = 'none';
        this.error.style.display = 'none';
      }
    }

    downloadFile() {
      if (!this.form.hasAttribute('data-download-file')) return

      const fileName = this.form.getAttribute('data-download-file');
      const downloadFileButton = this.success.querySelector('button.button');
      const a = document.createElement('a')

      a.href = `/reports/pci_ssc_reports_files/${fileName}`;
      a.download = fileName;
      a.click();

      downloadFileButton.addEventListener('click', () => {
        a.click();
      })
    }
  }

  const wrapper = document.querySelectorAll('[data-modal-window]');

  wrapper.forEach(item => {
    new PopupForm(item);
  })
})
