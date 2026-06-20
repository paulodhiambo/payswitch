import React, { useState } from 'react';
import { usePortalStore } from '../store/portalStore';
import { 
  Lock, 
  Mail, 
  Eye, 
  EyeOff, 
  ShieldCheck, 
  ArrowRight, 
  RefreshCw, 
  AlertTriangle,
  Server,
  Layers,
  Cpu
} from 'lucide-react';

export const Login: React.FC = () => {
  const login = usePortalStore((state) => state.login);
  
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim() || !password) {
      setError('Please fill in all authentication fields.');
      return;
    }
    
    setError(null);
    setIsLoading(true);
    
    try {
      const success = await login(email, password);
      if (!success) {
        setError('Invalid username or password. Please try again.');
        setIsLoading(false);
      }
    } catch (err) {
      setError('Connection to Authentik Identity Provider timed out.');
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#06080e] flex flex-col lg:flex-row relative overflow-hidden font-sans">
      
      {/* Decorative abstract glowing mesh background blobs */}
      <div className="absolute top-[-20%] left-[-10%] w-[50%] h-[50%] bg-indigo-900/10 rounded-full blur-[120px] pointer-events-none"></div>
      <div className="absolute bottom-[-10%] right-[-10%] w-[60%] h-[60%] bg-violet-900/15 rounded-full blur-[160px] pointer-events-none"></div>
      <div className="absolute top-[30%] right-[20%] w-[350px] h-[350px] bg-blue-900/5 rounded-full blur-[100px] pointer-events-none"></div>

      {/* Left panel: Product Branding & Security Context Info */}
      <div className="hidden lg:flex lg:w-7/12 flex-col justify-between p-12 relative z-10 border-r border-slate-800/40 bg-gradient-to-b from-[#080b13]/60 to-[#04060b]/90 backdrop-blur-sm">
        
        {/* Top Header branding */}
        <div className="flex items-center gap-3">
          <div className="bg-gradient-to-br from-indigo-500 to-violet-600 p-2.5 rounded-xl shadow-lg glow-indigo">
            <Cpu className="h-6 w-6 text-white" />
          </div>
          <div>
            <span className="font-extrabold text-lg tracking-wider bg-gradient-to-r from-indigo-200 via-slate-100 to-violet-200 bg-clip-text text-transparent">PAYSYS SWITCH</span>
            <span className="text-[10px] block text-gray-500 font-mono tracking-widest uppercase">Participant Console</span>
          </div>
        </div>

        {/* Hero Section */}
        <div className="my-auto max-w-xl space-y-6">
          <h1 className="text-4xl font-extrabold tracking-tight leading-tight text-white">
            Secure Interbank <span className="bg-gradient-to-r from-indigo-400 to-violet-400 bg-clip-text text-transparent">Settlement Gateway</span>
          </h1>
          <p className="text-gray-400 text-sm leading-relaxed">
            Authorized participant bank management portal. Clears high-volume ISO 20022 message orchestrations, provisions mTLS configurations, and monitors real-time clearing windows securely.
          </p>

          {/* Feature highlights grid */}
          <div className="grid grid-cols-2 gap-4 pt-6">
            <div className="glass-panel p-4 rounded-xl space-y-2 border-slate-800/60">
              <div className="h-8 w-8 rounded-lg bg-indigo-950/60 border border-indigo-900/40 flex items-center justify-center text-indigo-400">
                <ShieldCheck className="h-4 w-4" />
              </div>
              <p className="text-xs font-bold text-gray-200">mTLS Authentication</p>
              <p className="text-[10px] text-gray-400 font-mono">Kong & Authentik integration</p>
            </div>
            
            <div className="glass-panel p-4 rounded-xl space-y-2 border-slate-800/60">
              <div className="h-8 w-8 rounded-lg bg-emerald-950/60 border border-emerald-900/40 flex items-center justify-center text-emerald-400">
                <Layers className="h-4 w-4" />
              </div>
              <p className="text-xs font-bold text-gray-200">ISO 20022 Standards</p>
              <p className="text-[10px] text-gray-400 font-mono">Pacs.008 structured clearance</p>
            </div>
          </div>
        </div>

        {/* Footer specifications */}
        <div className="flex items-center gap-6 text-[10px] font-mono text-gray-500">
          <div className="flex items-center gap-1.5">
            <Server className="h-3.5 w-3.5" />
            <span>IDP: Authentik v2026.6</span>
          </div>
          <div>&bull;</div>
          <div>Compliant with Switch Security Directive v2</div>
        </div>
      </div>

      {/* Right panel: Login screen */}
      <div className="flex-1 flex flex-col justify-center items-center p-6 lg:p-12 relative z-10">
        
        {/* Mobile top logo branding */}
        <div className="lg:hidden flex items-center gap-2 mb-8">
          <div className="bg-gradient-to-br from-indigo-500 to-violet-600 p-2 rounded-lg">
            <Cpu className="h-5 w-5 text-white" />
          </div>
          <span className="font-bold text-base tracking-wider text-white">PAYSYS SWITCH</span>
        </div>

        <div className="w-full max-w-md space-y-6">
          
          {/* Main Login Card */}
          <div className="glass-panel p-8 rounded-2xl border-slate-800/80 shadow-2xl relative">
            
            {/* Authentik branding banner */}
            <div className="flex flex-col items-center text-center space-y-2 mb-6">
              <div className="h-12 w-12 rounded-full bg-violet-950/60 border border-violet-800/40 flex items-center justify-center text-violet-400 shadow-[0_0_15px_rgba(139,92,246,0.15)] animate-pulse-subtle">
                <ShieldCheck className="h-7 w-7 text-indigo-400" />
              </div>
              <div>
                <h2 className="text-lg font-bold text-white tracking-wide">Sign in to Paysys Switch Portal</h2>
                <p className="text-xs text-gray-400">Authenticated via Authentik Identity Provider</p>
              </div>
            </div>

            {/* Error alerts */}
            {error && (
              <div className="bg-rose-950/50 border border-rose-900/40 rounded-xl p-3 flex gap-2 items-start text-xs text-rose-300 animate-slide-up mb-4">
                <AlertTriangle className="h-4 w-4 text-rose-400 flex-shrink-0 mt-0.5" />
                <p>{error}</p>
              </div>
            )}

            {/* Form */}
            <form onSubmit={handleSubmit} className="space-y-4">
              
              {/* Email */}
              <div className="space-y-1.5">
                <label className="text-xs font-mono text-gray-400 block">Username or Email</label>
                <div className="relative">
                  <span className="absolute inset-y-0 left-0 pl-3 flex items-center text-gray-500">
                    <Mail className="h-4 w-4" />
                  </span>
                  <input
                    type="email"
                    required
                    disabled={isLoading}
                    placeholder="name@company.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full bg-[#070a12] border border-slate-800 rounded-xl pl-10 pr-4 py-2.5 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:shadow-[0_0_15px_rgba(99,102,241,0.25)] transition-all font-sans"
                  />
                </div>
              </div>

              {/* Password */}
              <div className="space-y-1.5">
                <div className="flex justify-between items-center">
                  <label className="text-xs font-mono text-gray-400 block">Password</label>
                  <a href="#" onClick={(e) => { e.preventDefault(); alert("Contact Switch Administrator at admin@payment-switch.example.com to reset sandbox credentials."); }} className="text-[10px] text-indigo-400 hover:underline">
                    Forgot Password?
                  </a>
                </div>
                <div className="relative">
                  <span className="absolute inset-y-0 left-0 pl-3 flex items-center text-gray-500">
                    <Lock className="h-4 w-4" />
                  </span>
                  <input
                    type={showPassword ? 'text' : 'password'}
                    required
                    disabled={isLoading}
                    placeholder="••••••••••••"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full bg-[#070a12] border border-slate-800 rounded-xl pl-10 pr-10 py-2.5 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:shadow-[0_0_15px_rgba(99,102,241,0.25)] transition-all font-mono"
                  />
                  <button
                    type="button"
                    disabled={isLoading}
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute inset-y-0 right-0 pr-3 flex items-center text-gray-500 hover:text-gray-300"
                  >
                    {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              {/* Submit Button */}
              <button
                type="submit"
                disabled={isLoading}
                className="w-full bg-gradient-to-r from-indigo-600 to-violet-600 hover:from-indigo-500 hover:to-violet-500 text-white font-bold text-sm py-2.5 rounded-xl shadow-lg hover:shadow-indigo-500/20 active:scale-[0.98] transition-all flex items-center justify-center gap-2 mt-6 cursor-pointer"
              >
                {isLoading ? (
                  <>
                    <RefreshCw className="h-4 w-4 animate-spin text-white" />
                    <span>Verifying tokens...</span>
                  </>
                ) : (
                  <>
                    <span>Sign In</span>
                    <ArrowRight className="h-4 w-4" />
                  </>
                )}
              </button>
            </form>
          </div>
          {/* SSO Notice Footer */}
          <p className="text-center text-[10px] text-gray-500">
            Powered by <a href="https://goauthentik.io/" target="_blank" rel="noreferrer" className="text-indigo-400 hover:underline">authentik</a>. Secure SSO authentication.
          </p>

        </div>
      </div>
    </div>
  );
};

export default Login;
