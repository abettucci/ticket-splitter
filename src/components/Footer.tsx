import { Github, Shield, MessageCircle } from "lucide-react";

export const Footer = () => {
  return (
    <footer className="bg-slate-950 text-white py-16">
      <div className="container mx-auto px-4">
        <div className="grid md:grid-cols-4 gap-8 mb-12">
          <div className="md:col-span-2">
            <div className="flex items-center gap-3 mb-6">
              <div className="bg-gradient-to-br from-teal-700 to-emerald-500 p-2 rounded-xl">
                <MessageCircle className="h-6 w-6 text-white" />
              </div>
              <span className="text-2xl font-bold">Splitter</span>
            </div>
            <p className="text-slate-400 mb-6 max-w-md">
              La forma más simple de ordenar gastos compartidos. Usalo online, en Telegram o en WhatsApp, sin anuncios ni seguimiento.
            </p>
            <div className="flex gap-4">
              <a 
                href="https://github.com/abettucci/ticket-splitter"
                target="_blank"
                rel="noopener noreferrer"
                className="bg-slate-800 p-3 rounded-xl hover:bg-slate-700 transition-colors"
              >
                <Github className="h-5 w-5" />
              </a>
              <div
                title="WhatsApp Web y grupos disponibles"
                className="bg-emerald-500/15 border border-emerald-400/30 p-3 rounded-xl text-emerald-300"
              >
                <MessageCircle className="h-5 w-5" />
              </div>
            </div>
          </div>
          
          <div>
            <h4 className="text-lg font-semibold mb-4">Producto</h4>
            <ul className="space-y-3 text-slate-400">
              <li><a href="#features" className="hover:text-emerald-300 transition-colors">Características</a></li>
              <li><a href="#how-it-works" className="hover:text-emerald-300 transition-colors">Cómo funciona</a></li>
              <li><span className="text-slate-300">Online, Telegram y WhatsApp</span></li>
              <li><span className="text-emerald-300">WhatsApp Web + grupos</span></li>
              <li><a href="https://github.com/abettucci/ticket-splitter" className="hover:text-emerald-300 transition-colors">Código fuente</a></li>
            </ul>
          </div>
          
          <div>
            <h4 className="text-lg font-semibold mb-4">Legal & Seguridad</h4>
            <ul className="space-y-3 text-slate-400">
              <li>
                <a href="/security" className="hover:text-emerald-300 transition-colors flex items-center gap-2">
                  <Shield className="h-4 w-4" />
                  Seguridad
                </a>
              </li>
              <li><a href="/privacy" className="hover:text-emerald-300 transition-colors">Privacidad</a></li>
              <li><a href="/terms" className="hover:text-emerald-300 transition-colors">Términos</a></li>
              <li><a href="https://github.com/abettucci/ticket-splitter/issues" className="hover:text-emerald-300 transition-colors">Reportar bug</a></li>
            </ul>
          </div>
        </div>
        
        {/* Badges de seguridad */}
        <div className="border-t border-slate-800 pt-8 mb-8">
          <div className="flex flex-wrap justify-center gap-4">
            <div className="bg-slate-900 px-4 py-2 rounded-lg flex items-center gap-2">
              <Shield className="h-4 w-4 text-green-400" />
              <span className="text-sm text-slate-300">Encriptación AES-256</span>
            </div>
            <div className="bg-slate-900 px-4 py-2 rounded-lg flex items-center gap-2">
              <Shield className="h-4 w-4 text-green-400" />
              <span className="text-sm text-slate-300">Protección de acceso</span>
            </div>
            <div className="bg-slate-900 px-4 py-2 rounded-lg flex items-center gap-2">
              <Shield className="h-4 w-4 text-green-400" />
              <span className="text-sm text-slate-300">TLS 1.3</span>
            </div>
            <div className="bg-slate-900 px-4 py-2 rounded-lg flex items-center gap-2">
              <Shield className="h-4 w-4 text-green-400" />
              <span className="text-sm text-slate-300">No tracking</span>
            </div>
          </div>
        </div>
        
        <div className="border-t border-slate-800 pt-8">
          <div className="flex flex-col md:flex-row justify-between items-center gap-4">
            <p className="text-slate-500">
              © {new Date().getFullYear()} Splitter. Open source bajo licencia MIT.
            </p>
            <p className="text-slate-500">
              Hecho con 🧉 en Argentina
            </p>
          </div>
        </div>
      </div>
    </footer>
  );
};
