import { Card, CardContent } from "@/components/ui/card";
import { UserPlus, Receipt, Calculator, CheckCircle } from "lucide-react";

const steps = [
  {
    icon: UserPlus,
    step: "1",
    title: "Agrega el bot",
    description: "Busca @SplitBot en Telegram y agrégalo a tu grupo. ¡Toma 5 segundos!"
  },
  {
    icon: Receipt,
    step: "2", 
    title: "Registra gastos",
    description: "Usa /nuevo_gasto Cena 15000 para registrar cualquier gasto del grupo."
  },
  {
    icon: Calculator,
    step: "3",
    title: "División automática", 
    description: "El bot calcula automáticamente cuánto debe cada persona del grupo."
  },
  {
    icon: CheckCircle,
    step: "4",
    title: "Marca pagos",
    description: "Cada miembro marca su pago cuando salda su deuda. Simple y transparente."
  }
];

export const HowItWorks = () => {
  return (
    <section className="py-20 bg-white">
      <div className="container mx-auto px-4">
        <div className="text-center mb-16">
          <h2 className="text-4xl lg:text-5xl font-bold text-slate-900 mb-6">
            ¿Cómo funciona
            <span className="bg-gradient-to-r from-[#0088cc] to-[#00a8e8] bg-clip-text text-transparent"> SplitBot</span>?
          </h2>
          <p className="text-xl text-slate-600 max-w-2xl mx-auto">
            En 4 pasos simples tendrás todos los gastos de tu grupo organizados y divididos automáticamente.
          </p>
        </div>
        
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-8">
          {steps.map((step, index) => (
            <div key={index} className="text-center relative">
              <Card className="bg-gradient-to-br from-slate-50 to-white shadow-lg border border-slate-100 p-8 hover:shadow-xl transition-all duration-300 group relative z-10">
                <CardContent className="p-0">
                  <div className="relative mb-6">
                    <div className="bg-gradient-to-br from-[#0088cc] to-[#00a8e8] p-4 rounded-2xl w-20 h-20 mx-auto flex items-center justify-center group-hover:scale-110 transition-transform duration-300 shadow-lg">
                      <step.icon className="h-8 w-8 text-white" />
                    </div>
                    <div className="absolute -top-2 -right-2 bg-amber-400 text-amber-900 rounded-full w-8 h-8 flex items-center justify-center text-sm font-bold shadow">
                      {step.step}
                    </div>
                  </div>
                  <h3 className="text-xl font-semibold text-slate-900 mb-4">{step.title}</h3>
                  <p className="text-slate-600 leading-relaxed">{step.description}</p>
                </CardContent>
              </Card>
              
              {index < steps.length - 1 && (
                <div className="hidden lg:block absolute top-1/2 -right-4 transform -translate-y-1/2 z-0">
                  <div className="w-8 h-1 bg-gradient-to-r from-[#0088cc] to-[#00a8e8] rounded-full"></div>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Comandos disponibles */}
        <div className="mt-20 bg-slate-900 rounded-3xl p-8 lg:p-12">
          <h3 className="text-2xl lg:text-3xl font-bold text-white text-center mb-8">
            Comandos disponibles
          </h3>
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4 max-w-4xl mx-auto">
            {[
              { cmd: "/nuevo_gasto [desc] [monto]", desc: "Crear un gasto" },
              { cmd: "/ver_gastos", desc: "Ver últimos gastos" },
              { cmd: "/dividir [id]", desc: "Dividir un gasto" },
              { cmd: "/mis_deudas", desc: "Ver tus deudas" },
              { cmd: "/pagar [id]", desc: "Marcar como pagado" },
              { cmd: "/balance", desc: "Ver balance grupal" },
            ].map((item, index) => (
              <div key={index} className="bg-slate-800 rounded-xl p-4">
                <code className="text-cyan-400 font-mono text-sm">{item.cmd}</code>
                <p className="text-slate-400 text-sm mt-1">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
};
