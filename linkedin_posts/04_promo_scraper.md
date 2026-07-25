# LinkedIn Post: Scraper de Promociones Bancarias

---

## El Problema

En Argentina, los supermercados tienen promociones bancarias que cambian constantemente: 30% con Santander en Carrefour, 25% con Galicia en Coto, etc. Pero encontrar la mejor promoción requiere:

1. Entrar a cada sitio de supermercado
2. Buscar la sección de promociones
3. Filtrar por tu banco
4. Leer los términos y condiciones (topes, exclusiones, días válidos)

Multiplicá eso por 7 supermercados y varios bancos. Imposible hacerlo cada semana.

---

## La Solución

Construí un **scraper automático** que extrae todas las promociones bancarias de los principales supermercados argentinos, parsea los términos y condiciones, y las presenta en un dashboard web.

**Supermercados soportados:**
- Carrefour
- Coto Digital
- Disco
- Jumbo
- Día
- Walmart
- Changomás

**Features:**
- **Playwright-Stealth**: Anti-detección para sitios con protección
- **Parser de T&C**: Extrae topes, exclusiones, requisitos
- **SQLite**: Base de datos local con historial
- **Dashboard Streamlit**: Visualización y filtros
- **Ejecución programada**: Cron o GitHub Actions

---

## Stack Tecnológico

```
Scraping:     Playwright + playwright-stealth
Parsing:      BeautifulSoup + regex patterns
Database:     SQLite
Dashboard:    Streamlit
Scheduling:   Cron / GitHub Actions
Language:     Python
```

---

## Arquitectura

```
promo-scraper/
├── scraper.py              # Script principal
├── scrapers/               # Scrapers específicos por supermercado
│   ├── carrefour.py
│   ├── coto.py
│   └── ...
├── database.py             # Gestión de BD
├── terms_parser.py         # Parser de T&C
├── dashboard.py            # Dashboard Streamlit
└── data/
    └── promotions.db       # SQLite database
```

---

## Desafíos Técnicos

1. **Anti-bot protection**: Algunos sitios detectan Playwright. Solución: playwright-stealth + delays aleatorios
2. **Contenido dinámico**: SPAs que cargan con JavaScript. Solución: wait for selectors + scroll automático
3. **Estructura variable**: Cada supermercado tiene HTML diferente. Solución: scraper específico por sitio
4. **T&C complejos**: Texto no estructurado con topes y exclusiones. Solución: regex patterns + NLP básico

---

## Texto para LinkedIn (copiar y pegar)

```
En Argentina, encontrar la mejor promoción bancaria para el supermercado es un trabajo de tiempo completo.

30% con Santander en Carrefour, 25% con Galicia en Coto, pero solo los martes, con tope de $5000, excluyendo bebidas alcohólicas...

El problema:
• 7+ supermercados con promociones diferentes
• Términos y condiciones enterrados en letra chica
• Promociones que cambian cada semana
• Imposible comparar manualmente

La solución:
• Scraper automático con Playwright (anti-detección)
• Parser de términos y condiciones
• Dashboard Streamlit para visualizar y filtrar
• Ejecución programada con GitHub Actions

Desafíos técnicos:
• Anti-bot protection → playwright-stealth
• Contenido dinámico (SPAs) → wait + scroll automático
• Estructura HTML variable → scraper específico por sitio
• T&C no estructurados → regex + NLP básico

Stack: Python, Playwright, BeautifulSoup, SQLite, Streamlit

Ahora sé exactamente qué día ir a qué supermercado con qué tarjeta.

#WebScraping #Python #Playwright #Automation #Argentina #SideProject
```

---

## Hashtags Recomendados

#WebScraping #Python #Playwright #Automation #DataExtraction #Streamlit #Argentina #FinTech #SideProject #BeautifulSoup
