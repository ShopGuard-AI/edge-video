// Custom JavaScript para Edge Video Documentation

document.addEventListener('DOMContentLoaded', function() {
  // Add copy button feedback
  document.querySelectorAll('.md-clipboard').forEach(function(button) {
    button.addEventListener('click', function() {
      const icon = button.querySelector('svg');
      if (icon) {
        icon.style.color = '#4caf50';
        setTimeout(() => {
          icon.style.color = '';
        }, 2000);
      }
    });
  });

  // Smooth scroll para âncoras
  document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function (e) {
      const href = this.getAttribute('href');
      if (href !== '#') {
        e.preventDefault();
        const target = document.querySelector(href);
        if (target) {
          target.scrollIntoView({
            behavior: 'smooth',
            block: 'start'
          });
        }
      }
    });
  });

  // External links open in new tab
  document.querySelectorAll('a[href^="http"]').forEach(link => {
    if (!link.hostname.includes(window.location.hostname)) {
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
    }
  });

  // Add badges to version numbers
  document.querySelectorAll('code').forEach(code => {
    const text = code.textContent;
    if (/^v?\d+\.\d+\.\d+$/.test(text)) {
      code.classList.add('badge', 'badge-info');
    }
  });
});

// Analytics (opcional - configurar se necessário)
// window.dataLayer = window.dataLayer || [];
// function gtag(){dataLayer.push(arguments);}
// gtag('js', new Date());
// gtag('config', 'G-XXXXXXXXXX');
