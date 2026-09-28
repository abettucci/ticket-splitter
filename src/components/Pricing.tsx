import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Check, Sparkles, Gift, MessageCircle } from "lucide-react";
import { Link } from "react-router-dom";

export const Pricing = () => {
  const features = [
    "Gastos, grupos y miembros sin límites",
    "División automática y balance claro",
    "Historial de gastos y pagos",
    "Recordatorios privados",
    "Telegram y WhatsApp",
    "Menú simple y lenguaje natural",
    "Sin anuncios ni seguimiento",
    "Tus datos, siempre privados",
  ];

  return (
    <section className="py-20 bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
      <div className="container mx-auto px-4">
        <div className="text-center mb-16">
          <div className="inline-flex items-center gap-2 bg-green-500/20 px-4 py-2 rounded-full mb-6">
            <Gift className="h-5 w-5 text-green-400" />
            <span className="text-green-400 font-medium">MVP gratuito</span>
          </div>
          
          <h2 className="text-4xl lg:text-5xl font-bold text-white mb-6">
            Sin planes, sin vueltas,
            <span className="bg-gradient-to-r from-emerald-400 to-teal-300 bg-clip-text text-transparent"> para ordenar las cuentas</span>
          </h2>
          <p className="text-xl text-slate-300 max-w-2xl mx-auto">
            Splitter está pensado para que organizar gastos entre amigos, familia o viajes no te agregue otro problema.
          </p>
        </div>
        
        <div className="max-w-2xl mx-auto">
          <Card className="bg-gradient-to-br from-slate-800 to-slate-900 border-2 border-emerald-500/70 shadow-2xl shadow-emerald-500/15 px-8 pb-8 pt-14 relative overflow-visible">
            {/* Decoración */}
            <div className="absolute top-0 right-0 w-40 h-40 bg-gradient-to-br from-emerald-400/20 to-transparent rounded-full blur-3xl" />
            
            <div className="absolute -top-4 left-1/2 -translate-x-1/2 bg-gradient-to-r from-teal-700 to-emerald-500 text-white px-6 py-2 rounded-full flex items-center gap-2 shadow-lg whitespace-nowrap">
              <Sparkles className="h-4 w-4" />
              <span className="font-semibold">Gratis durante el MVP</span>
            </div>
            
            <CardHeader className="p-0 mb-8 text-center pt-4">
              <CardTitle className="text-3xl font-bold text-white mb-2">Splitter</CardTitle>
              <div className="mb-4">
                <span className="text-6xl font-bold text-emerald-400">$0</span>
                <span className="text-slate-400 ml-2 text-xl">/mes</span>
              </div>
              <p className="text-slate-300">Lo esencial para compartir gastos sin perder tiempo en cuentas.</p>
            </CardHeader>
            
            <CardContent className="p-0">
              <div className="grid sm:grid-cols-2 gap-4 mb-8">
                {features.map((feature, index) => (
                  <div key={index} className="flex items-center gap-3">
                    <div className="bg-green-500/20 p-1 rounded-full">
                      <Check className="h-4 w-4 text-green-400" />
                    </div>
                    <span className="text-slate-200">{feature}</span>
                  </div>
                ))}
              </div>
              
              <Button asChild className="w-full bg-gradient-to-r from-teal-700 to-emerald-500 hover:from-teal-800 hover:to-emerald-600 text-white text-lg py-6 rounded-xl shadow-lg shadow-emerald-500/20" size="lg">
                <Link to="/dividir">Probar Splitter</Link>
              </Button>
            </CardContent>
          </Card>
        </div>
        
        <div className="mt-14 flex flex-wrap items-center justify-center gap-3 text-sm text-slate-300">
          <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-2"><MessageCircle className="h-4 w-4 text-emerald-300" /> Telegram y WhatsApp</span>
          <span className="rounded-full border border-white/10 bg-white/5 px-4 py-2">Sin anuncios</span>
          <span className="rounded-full border border-white/10 bg-white/5 px-4 py-2">Sin planillas</span>
        </div>
      </div>
    </section>
  );
};
