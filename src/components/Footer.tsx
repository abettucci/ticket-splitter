import { Send, Github, Twitter, Shield } from "lucide-react";

export const Footer = () => {
  return (
    <footer className="bg-slate-950 text-white py-16">
      <div className="container mx-auto px-4">
        <div className="grid md:grid-cols-4 gap-8 mb-12">
          <div className="md:col-span-2">
            <div className="flex items-center gap-3 mb-6">
              <div className="bg-gradient-to-br from-[#0088cc] to-cyan-500 p-2 rounded-xl">
                <Send className="h-6 w-6 text-white" />
              </div>
              <span className="text-2xl font-bold">SplitBot</span>
            </div>
            <p className="text-slate-400 mb-6 max-w-md">
              La forma más simple de dividir gastos en Telegram. 100% gratis, sin ads, sin tracking. Open source.
            </p>
            <div className="flex gap-4">
              <a 
                href="https://github.com/tu-usuario/group-split-bot" 
                target="_blank"
                rel="noopener noreferrer"
                className="bg-slate-800 p-3 rounded-xl hover:bg-slate-700 transition-colors"
              >
                <Github className="h-5 w-5" />
              </a>
              <a 
                href="https://twitter.com/tu-usuario" 
                target="_blank"
                rel="noopener noreferrer"
                className="bg-slate-800 p-3 rounded-xl hover:bg-slate-700 transition-colors"
              >
                <Twitter className="h-5 w-5" />
              </a>
              <a 
                href="https://t.me/YourSplitBot" 
                target="_blank"
                rel="noopener noreferrer"
                className="bg-slate-800 p-3 rounded-xl hover:bg-slate-700 transition-colors"
              >
                <Send className="h-5 w-5" />
              </a>
            </div>
          </div>
          
          <div>
            <h4 className="text-lg font-semibold mb-4">Producto</h4>
            <ul className="space-y-3 text-slate-400">
              <li><a href="#features" className="hover:text-[#0088cc] transition-colors">Características</a></li>
              <li><a href="#how-it-works" className="hover:text-[#0088cc] transition-colors">Cómo funciona</a></li>
              <li><a href="https://t.me/YourSplitBot" className="hover:text-[#0088cc] transition-colors">Abrir bot</a></li>
              <li><a href="https://github.com/tu-usuario/group-split-bot" className="hover:text-[#0088cc] transition-colors">Código fuente</a></li>
            </ul>
          </div>
          
          <div>
            <h4 className="text-lg font-semibold mb-4">Legal & Seguridad</h4>
            <ul className="space-y-3 text-slate-400">
              <li>
                <a href="/security" className="hover:text-[#0088cc] transition-colors flex items-center gap-2">
                  <Shield className="h-4 w-4" />
                  Seguridad
                </a>
              </li>
              <li><a href="/privacy" className="hover:text-[#0088cc] transition-colors">Privacidad</a></li>
              <li><a href="/terms" className="hover:text-[#0088cc] transition-colors">Términos</a></li>
              <li><a href="https://github.com/tu-usuario/group-split-bot/issues" className="hover:text-[#0088cc] transition-colors">Reportar bug</a></li>
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
              <span className="text-sm text-slate-300">AWS WAF Protection</span>
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
              © {new Date().getFullYear()} SplitBot. Open source bajo licencia MIT.
            </p>
            <p className="text-slate-500">
              Hecho con 💙 en Argentina
            </p>
          </div>
        </div>
      </div>
    </footer>
  );
};
